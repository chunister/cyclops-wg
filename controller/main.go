// cyclops-wg controller: sobe os gateways (ativos e reserva), mantém o
// balanceador atualizado e promove uma reserva quando um ativo cai.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const papelAtivo = "ativo"
const papelReserva = "reserva"

type Slot struct {
	Nome     string
	Servidor string
	IP       string
	Papel    string
	Falhas   int
	EmReparo bool `json:"-"`
}

type Controlador struct {
	cfg      Config
	docker   *Docker
	cooldown *Cooldown
	catalogo []string

	mu    sync.Mutex
	slots []*Slot
}

func main() {
	cfg := carregaConfig()
	if cfg.WGPrivateKey == "" {
		log.Fatal("defina WG_PRIVATE_KEY no ambiente")
	}

	docker, err := NovoDocker(cfg)
	if err != nil {
		log.Fatalf("docker: %v", err)
	}

	ctx := context.Background()
	if err := docker.GaranteRede(ctx); err != nil {
		log.Fatalf("rede: %v", err)
	}

	if err := ConstroiImagens(cfg, cfg.RepoRoot); err != nil {
		log.Fatalf("build: %v", err)
	}

	catalogo, err := Catalogo(cfg.ServersDir)
	if err != nil {
		log.Fatalf("catálogo de servidores: %v", err)
	}
	total := cfg.GatewayCount + cfg.SpareCount
	if len(catalogo) < total {
		log.Fatalf("catálogo tem %d servidores, preciso de %d (GATEWAY_COUNT + SPARE_COUNT)", len(catalogo), total)
	}

	ctrl := &Controlador{
		cfg:      cfg,
		docker:   docker,
		cooldown: AbreCooldown(cfg.StateFile),
		catalogo: catalogo,
	}

	if err := ctrl.sobeTudo(ctx); err != nil {
		log.Fatalf("subida inicial: %v", err)
	}

	ctrl.serveStatus()

	if !cfg.Dashboard {
		ticker := time.NewTicker(time.Duration(cfg.HealthInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ctrl.cicloDeReconciliacao(ctx)
		}
		return
	}

	inicio := time.Now()
	dash := NovoDashboard()
	log.SetOutput(dash)

	go func() {
		ticker := time.NewTicker(time.Duration(cfg.HealthInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ctrl.cicloDeReconciliacao(ctx)
		}
	}()

	go func() {
		redesenha := time.NewTicker(time.Second)
		defer redesenha.Stop()
		for range redesenha.C {
			dash.Atualiza(inicio, ctrl.CopiaSlots())
		}
	}()

	go func() {
		redesenha := time.NewTicker(2 * time.Second)
		defer redesenha.Stop()
		for range redesenha.C {
			conexoes, err := ConexoesRecentes(cfg.BalancerAddr)
			if err != nil {
				continue
			}
			dash.AtualizaConexoes(conexoes)
		}
	}()

	dash.Atualiza(inicio, ctrl.CopiaSlots())
	if err := dash.Executa(); err != nil {
		fmt.Printf("dashboard: %v\n", err)
		os.Exit(1)
	}
}

// CopiaSlots devolve uma cópia dos slots atuais, segura para o dashboard
// desenhar sem segurar o mutex do controlador durante o redesenho.
func (c *Controlador) CopiaSlots() []*Slot {
	c.mu.Lock()
	defer c.mu.Unlock()
	copia := make([]*Slot, len(c.slots))
	for i, s := range c.slots {
		cp := *s
		copia[i] = &cp
	}
	return copia
}

// sobeTudo cria do zero os N+M gateways e o balanceador, e espera o
// handshake inicial de cada um antes de publicar a lista no balanceador.
func (c *Controlador) sobeTudo(ctx context.Context) error {
	usados := map[string]bool{}
	total := c.cfg.GatewayCount + c.cfg.SpareCount

	c.docker.RemoveSeExistir(ctx, "cyclops-balancer")
	for i := 1; i <= total; i++ {
		c.docker.RemoveSeExistir(ctx, fmt.Sprintf("cyclops-gw-%d", i))
	}

	for i := 1; i <= total; i++ {
		servidor, err := Escolhe(c.catalogo, usados, c.cooldown, time.Duration(c.cfg.ServerCooldown)*time.Second)
		if err != nil {
			return err
		}
		usados[servidor] = true

		nome := fmt.Sprintf("cyclops-gw-%d", i)
		log.Printf("subindo %s com %s", nome, servidor)
		if err := c.docker.CriaGateway(ctx, nome, servidor); err != nil {
			return fmt.Errorf("criar %s: %w", nome, err)
		}

		papel := papelAtivo
		if i > c.cfg.GatewayCount {
			papel = papelReserva
		}
		c.slots = append(c.slots, &Slot{Nome: nome, Servidor: servidor, Papel: papel})
	}

	if err := c.docker.CriaBalanceador(ctx); err != nil {
		return fmt.Errorf("criar balanceador: %w", err)
	}

	log.Printf("esperando os túneis subirem...")
	time.Sleep(10 * time.Second)
	for _, s := range c.slots {
		ip, err := c.docker.IPNaRede(ctx, s.Nome)
		if err != nil {
			log.Printf("sem IP ainda para %s: %v", s.Nome, err)
			continue
		}
		s.IP = ip
	}

	return c.publicaAtivos()
}

// publicaAtivos manda ao balanceador a lista atual de IP:1090 dos slots ativos.
func (c *Controlador) publicaAtivos() error {
	c.mu.Lock()
	var enderecos []string
	for _, s := range c.slots {
		if s.Papel == papelAtivo && s.IP != "" {
			enderecos = append(enderecos, s.IP+":1090")
		}
	}
	c.mu.Unlock()

	return AtualizaBalanceador(c.cfg.BalancerAddr, enderecos)
}

// cicloDeReconciliacao testa todos os slots e promove uma reserva saudável
// no lugar de qualquer ativo que tenha acumulado HEALTH_FAILS falhas seguidas.
func (c *Controlador) cicloDeReconciliacao(ctx context.Context) {
	c.mu.Lock()
	slots := append([]*Slot{}, c.slots...)
	c.mu.Unlock()

	for _, s := range slots {
		if s.IP == "" {
			ip, err := c.docker.IPNaRede(ctx, s.Nome)
			if err != nil {
				continue
			}
			s.IP = ip
		}
		err := testaGateway(s.IP+":1090", "1.1.1.1:443", 8*time.Second)
		c.mu.Lock()
		if err != nil {
			s.Falhas++
			log.Printf("%s (%s) falhou a verificação (%d): %v", s.Nome, s.Papel, s.Falhas, err)
		} else {
			s.Falhas = 0
		}
		c.mu.Unlock()
	}

	c.promoveSeNecessario(ctx)
	c.curaReservasDoentes(ctx)
}

// curaReservasDoentes pega qualquer reserva que acumulou HEALTH_FAILS falhas
// (por exemplo, um gateway que já foi demovido e caiu de novo) e manda
// reiniciar e trocar de servidor em segundo plano, do mesmo jeito que uma
// demoção recente. Sem isso, uma reserva que falhar fora do momento da
// promoção ficava largada para sempre, nunca mais tentando se recuperar.
func (c *Controlador) curaReservasDoentes(ctx context.Context) {
	c.mu.Lock()
	var doentes []*Slot
	for _, s := range c.slots {
		if s.Papel == papelReserva && s.Falhas >= c.cfg.HealthFails && !s.EmReparo {
			s.EmReparo = true
			s.Falhas = 0
			doentes = append(doentes, s)
		}
	}
	c.mu.Unlock()

	for _, s := range doentes {
		go func(s *Slot) {
			c.reequilibraServidor(ctx, s)
			c.mu.Lock()
			s.EmReparo = false
			c.mu.Unlock()
		}(s)
	}
}

func (c *Controlador) promoveSeNecessario(ctx context.Context) {
	c.mu.Lock()
	var doente *Slot
	var reservaBoa *Slot
	for _, s := range c.slots {
		if s.Papel == papelAtivo && s.Falhas >= c.cfg.HealthFails && doente == nil {
			doente = s
		}
		if s.Papel == papelReserva && s.Falhas == 0 && !s.EmReparo && reservaBoa == nil {
			reservaBoa = s
		}
	}
	if doente == nil {
		c.mu.Unlock()
		return
	}
	if reservaBoa == nil {
		c.mu.Unlock()
		log.Printf("%s doente, mas sem reserva saudável; reiniciando o container", doente.Nome)
		if err := c.docker.ReiniciaGateway(ctx, doente.Nome); err != nil {
			log.Printf("reinício de %s falhou: %v", doente.Nome, err)
		}
		return
	}

	log.Printf("promovendo %s no lugar de %s", reservaBoa.Nome, doente.Nome)
	doente.Papel = papelReserva
	reservaBoa.Papel = papelAtivo
	doente.Falhas = 0
	doente.EmReparo = true
	c.mu.Unlock()

	if err := c.publicaAtivos(); err != nil {
		log.Printf("atualizar balanceador: %v", err)
	}

	go func() {
		c.reequilibraServidor(ctx, doente)
		c.mu.Lock()
		doente.EmReparo = false
		c.mu.Unlock()
	}()
}

// reequilibraServidor troca o servidor do slot que acabou de virar reserva,
// em segundo plano, para ele voltar a ficar disponível para uma promoção futura.
func (c *Controlador) reequilibraServidor(ctx context.Context, s *Slot) {
	c.mu.Lock()
	usados := map[string]bool{}
	for _, outro := range c.slots {
		if outro != s {
			usados[outro.Servidor] = true
		}
	}
	c.mu.Unlock()

	novoServidor, err := Escolhe(c.catalogo, usados, c.cooldown, time.Duration(c.cfg.ServerCooldown)*time.Second)
	if err != nil {
		log.Printf("sem servidor livre para %s agora: %v", s.Nome, err)
		return
	}

	rodando, err := c.docker.EstaRodando(ctx, s.Nome)
	if err != nil {
		log.Printf("não consegui checar se %s está rodando: %v", s.Nome, err)
		return
	}
	if !rodando {
		log.Printf("%s não está rodando (provavelmente caiu); reiniciando antes de trocar o servidor", s.Nome)
		if err := c.docker.ReiniciaGateway(ctx, s.Nome); err != nil {
			log.Printf("reinício de %s falhou: %v", s.Nome, err)
			return
		}
		if !esperaContainerRodando(ctx, c.docker, s.Nome, 30*time.Second) {
			log.Printf("%s não voltou a rodar a tempo", s.Nome)
			return
		}
	}

	var trocaErr error
	for tentativa := 1; tentativa <= 5; tentativa++ {
		trocaErr = c.docker.TrocaServidor(ctx, s.Nome, novoServidor, 30*time.Second)
		if trocaErr == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if trocaErr != nil {
		log.Printf("switch-server em %s falhou: %v", s.Nome, trocaErr)
		return
	}

	c.mu.Lock()
	s.Servidor = novoServidor
	c.mu.Unlock()
	log.Printf("%s agora aponta para %s", s.Nome, novoServidor)
}

func (c *Controlador) serveStatus() {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		defer c.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c.slots)
	})
	go func() {
		if err := http.ListenAndServe(":"+c.cfg.ControllerPort, mux); err != nil {
			log.Printf("status do controlador: %v", err)
		}
	}()
	log.Printf("status do controlador em :%s/status", c.cfg.ControllerPort)
}
