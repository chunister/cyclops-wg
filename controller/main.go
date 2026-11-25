// cyclops-wg controller: sobe os gateways (ativos e reserva), mantém o
// balanceador atualizado e promove uma reserva quando um ativo cai.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const papelAtivo = "ativo"
const papelReserva = "reserva"

type Slot struct {
	Nome        string
	Servidor    string
	IP          string
	IPSaida     string
	Papel       string
	Falhas      int
	PingMs      int64 // último round-trip do health check bem-sucedido; 0 = nunca medido
	UltimaTroca time.Time
	EmReparo    bool `json:"-"`
}

type Controlador struct {
	cfg      Config
	docker   *Docker
	cooldown *Cooldown
	catalogo []string

	mu           sync.Mutex
	slots        []*Slot
	rotacionando bool
}

func main() {
	cfg := carregaConfig()
	DefineIdioma(idiomaDeString(cfg.Lang))
	if cfg.WGPrivateKey == "" {
		log.Fatal(T("err_wg_key_required"))
	}

	docker, err := NovoDocker(cfg)
	if err != nil {
		log.Fatal(T("err_docker", err))
	}

	ctx := context.Background()
	if err := docker.GaranteRede(ctx); err != nil {
		log.Fatal(T("err_network", err))
	}

	if err := ConstroiImagens(cfg, cfg.RepoRoot); err != nil {
		log.Fatal(T("err_build", err))
	}

	catalogo, err := Catalogo(cfg.ServersDir)
	if err != nil {
		log.Fatal(T("err_catalog", err))
	}
	total := cfg.GatewayCount + cfg.SpareCount
	if len(catalogo) < total {
		log.Fatal(T("err_catalog_too_small", len(catalogo), total))
	}

	ctrl := &Controlador{
		cfg:      cfg,
		docker:   docker,
		cooldown: AbreCooldown(cfg.StateFile),
		catalogo: catalogo,
	}

	if err := ctrl.sobeTudo(ctx); err != nil {
		log.Fatal(T("err_initial_boot", err))
	}

	ctrl.serveStatus()

	if cfg.FullRotateInterval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.FullRotateInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				ctrl.rotacionaTodos(ctx)
			}
		}()
	}

	if !cfg.Dashboard {
		destinos := []io.Writer{os.Stderr}
		if arquivo, ok := abreArquivoDeLog(cfg.LogFile); ok {
			defer arquivo.Close()
			destinos = append(destinos, arquivo)
		}
		log.SetOutput(io.MultiWriter(destinos...))

		ticker := time.NewTicker(time.Duration(cfg.HealthInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ctrl.cicloDeReconciliacao(ctx)
		}
		return
	}

	inicio := time.Now()
	dash := NovoDashboard()
	destinos := []io.Writer{dash}
	if arquivo, ok := abreArquivoDeLog(cfg.LogFile); ok {
		defer arquivo.Close()
		destinos = append(destinos, arquivo)
	}
	log.SetOutput(io.MultiWriter(destinos...))
	dash.DefinirAoRenovar(func() { go ctrl.rotacionaTodos(ctx) })

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
			dash.AtualizaRecursos()
		}
	}()

	go func() {
		redesenha := time.NewTicker(2 * time.Second)
		defer redesenha.Stop()
		for range redesenha.C {
			if conexoes, err := ConexoesRecentes(cfg.BalancerAddr); err == nil {
				ipSaida := map[string]string{}
				for _, s := range ctrl.CopiaSlots() {
					ipSaida[s.IP] = s.IPSaida
				}
				dash.AtualizaConexoes(conexoes, ipSaida)
			}
			if upload, download, err := Throughput(cfg.BalancerAddr); err == nil {
				dash.AtualizaThroughput(upload, download)
			}
		}
	}()

	if err := dash.Executa(); err != nil {
		fmt.Printf("dashboard: %v\n", err)
		os.Exit(1)
	}
}

// abreArquivoDeLog abre (ou cria) o arquivo de log em modo de acréscimo, pra
// dar pra revisar o que aconteceu mesmo depois que o painel de eventos (que
// só guarda as últimas 500 linhas) ou o terminal ficarem difíceis de ler,
// como num teste de carga. caminho vazio desliga o recurso.
func abreArquivoDeLog(caminho string) (*os.File, bool) {
	if caminho == "" {
		return nil, false
	}
	arquivo, err := os.OpenFile(caminho, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Print(T("log_file_open_failed", caminho, err))
		return nil, false
	}
	return arquivo, true
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
		log.Print(T("log_bringing_up_gateway", nome, servidor))
		if err := c.docker.CriaGateway(ctx, nome, servidor); err != nil {
			return fmt.Errorf("criar %s: %w", nome, err)
		}

		papel := papelAtivo
		if i > c.cfg.GatewayCount {
			papel = papelReserva
		}
		c.slots = append(c.slots, &Slot{Nome: nome, Servidor: servidor, Papel: papel, UltimaTroca: time.Now()})
	}

	if err := c.docker.CriaBalanceador(ctx); err != nil {
		return fmt.Errorf("criar balanceador: %w", err)
	}

	log.Print(T("log_waiting_tunnels"))
	time.Sleep(10 * time.Second)
	for _, s := range c.slots {
		ip, err := c.docker.IPNaRede(ctx, s.Nome)
		if err != nil {
			log.Print(T("log_no_ip_yet", s.Nome, err))
			continue
		}
		s.IP = ip
		if saida, err := buscaIPSaida(ip+":1090", 8*time.Second); err == nil {
			s.IPSaida = saida
		} else {
			log.Print(T("log_exit_ip_lookup_failed", s.Nome, err))
		}
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
		duracao, err := testaGateway(s.IP+":1090", c.cfg.HealthTarget, time.Duration(c.cfg.DialTimeout)*time.Second)
		c.mu.Lock()
		if err != nil {
			s.Falhas++
			log.Print(T("log_health_check_failed", s.Nome, s.Papel, s.Falhas, err))
		} else {
			s.Falhas = 0
			s.PingMs = duracao.Milliseconds()
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
		log.Print(T("log_sick_no_spare", doente.Nome))
		if err := c.docker.ReiniciaGateway(ctx, doente.Nome); err != nil {
			log.Print(T("log_restart_failed", doente.Nome, err))
		}
		return
	}

	log.Print(T("log_promoting", reservaBoa.Nome, doente.Nome))
	doente.Papel = papelReserva
	reservaBoa.Papel = papelAtivo
	doente.Falhas = 0
	doente.EmReparo = true
	c.mu.Unlock()

	if err := c.publicaAtivos(); err != nil {
		log.Print(T("log_balancer_update_failed", err))
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
		log.Print(T("log_no_free_server", s.Nome, err))
		return
	}

	rodando, err := c.docker.EstaRodando(ctx, s.Nome)
	if err != nil {
		log.Print(T("log_running_check_failed", s.Nome, err))
		return
	}
	if !rodando {
		log.Print(T("log_not_running_restarting", s.Nome))
		if err := c.docker.ReiniciaGateway(ctx, s.Nome); err != nil {
			log.Print(T("log_restart_failed", s.Nome, err))
			return
		}
		if !esperaContainerRodando(ctx, c.docker, s.Nome, 30*time.Second) {
			log.Print(T("log_not_running_in_time", s.Nome))
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
		log.Print(T("log_switch_server_failed", s.Nome, trocaErr))
		return
	}

	c.mu.Lock()
	s.Servidor = novoServidor
	s.UltimaTroca = time.Now()
	c.mu.Unlock()
	log.Print(T("log_now_points_to", s.Nome, novoServidor))

	if saida, err := buscaIPSaida(s.IP+":1090", 8*time.Second); err == nil {
		c.mu.Lock()
		s.IPSaida = saida
		c.mu.Unlock()
	} else {
		log.Print(T("log_new_exit_ip_lookup_failed", s.Nome, err))
	}
}

// rotacionaTodos troca o servidor de todos os slots de uma vez, não só dos
// que falharam, pra variar o conjunto de IPs de saída mesmo sem queda.
// Marca todos como EmReparo durante a troca pra promoveSeNecessario não
// tentar promover ninguém no meio da operação, e ignora um novo pedido
// se uma rotação já estiver em andamento.
func (c *Controlador) rotacionaTodos(ctx context.Context) {
	c.mu.Lock()
	if c.rotacionando {
		c.mu.Unlock()
		log.Print(T("log_rotation_already_running"))
		return
	}
	c.rotacionando = true
	slots := append([]*Slot{}, c.slots...)
	for _, s := range slots {
		s.EmReparo = true
	}
	c.mu.Unlock()

	log.Print(T("log_rotation_start", len(slots)))
	for _, s := range slots {
		c.reequilibraServidor(ctx, s)
	}
	log.Print(T("log_rotation_done"))

	c.mu.Lock()
	for _, s := range slots {
		s.EmReparo = false
	}
	c.rotacionando = false
	c.mu.Unlock()
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
			log.Print(T("log_status_server_error", err))
		}
	}()
	log.Print(T("log_status_server_listening", c.cfg.ControllerPort))
}
