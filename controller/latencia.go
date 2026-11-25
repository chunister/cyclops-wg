package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// pingTimeout é o prazo de cada ping individual. É bem mais curto que o
// DIAL_TIMEOUT (que serve pra uma conexão SOCKS5 completa através de um
// túnel já de pé): aqui é só um ICMP cru até o endpoint do WireGuard, antes
// de qualquer túnel existir.
const pingTimeout = 2 * time.Second

// pesoAmostraNova é o peso (alpha) da média móvel exponencial: quanto cada
// ping novo pesa sobre a média acumulada. Mais alto reage mais rápido a uma
// rota que mudou; mais baixo suaviza mais picos isolados. 0.3 foi escolhido
// como meio-termo razoável, não veio de nenhuma medição específica.
const pesoAmostraNova = 0.3

var regexPingTempo = regexp.MustCompile(`time[=<]([0-9.]+)\s*ms`)

// lerEndpointHost lê o campo "Endpoint = host:porta" do [Peer] de um .conf
// de servidor e devolve só o host, sem a porta.
func lerEndpointHost(caminho string) (string, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return "", err
	}
	for _, linha := range strings.Split(string(dados), "\n") {
		linha = strings.TrimSpace(linha)
		if !strings.HasPrefix(linha, "Endpoint") {
			continue
		}
		partes := strings.SplitN(linha, "=", 2)
		if len(partes) != 2 {
			continue
		}
		endpoint := strings.TrimSpace(partes[1])
		host, _, err := net.SplitHostPort(endpoint)
		if err != nil {
			return "", err
		}
		return host, nil
	}
	return "", fmt.Errorf("campo Endpoint não encontrado em %s", caminho)
}

// pingMs manda um ICMP echo pro host e devolve o round-trip em milissegundos.
// Usa o "ping" do sistema em vez de um socket ICMP cru: mais simples, e o
// controlador já roda como root (precisa pra falar com o Docker), então a
// permissão nunca é o problema.
func pingMs(host string, timeout time.Duration) (int64, error) {
	segundos := int(timeout.Seconds())
	if segundos < 1 {
		segundos = 1
	}
	saida, err := exec.Command("ping", "-c", "1", "-W", strconv.Itoa(segundos), host).Output()
	if err != nil {
		return 0, fmt.Errorf("ping pra %s falhou: %w", host, err)
	}
	achado := regexPingTempo.FindSubmatch(saida)
	if achado == nil {
		return 0, fmt.Errorf("resposta do ping a %s sem tempo reconhecível", host)
	}
	ms, err := strconv.ParseFloat(string(achado[1]), 64)
	if err != nil {
		return 0, err
	}
	return int64(ms), nil
}

// amostraLatencia é a média móvel exponencial do ping de um servidor,
// persistida entre execuções.
type amostraLatencia struct {
	Media    float64 `json:"media_ms"`
	Amostras int     `json:"amostras"`
}

// LatenciaCatalogo guarda, em disco, a média de ping de cada servidor do
// catálogo. Quem decide quando reping-ar é quem chama Atualiza; esta
// estrutura só guarda e consulta.
type LatenciaCatalogo struct {
	caminho string
	mu      sync.Mutex
	dados   map[string]amostraLatencia
}

// AbreLatencia carrega o arquivo de médias se ele já existir; se não existir
// ainda (primeira execução), começa vazio.
func AbreLatencia(caminho string) *LatenciaCatalogo {
	l := &LatenciaCatalogo{caminho: caminho, dados: map[string]amostraLatencia{}}
	dados, err := os.ReadFile(caminho)
	if err == nil {
		json.Unmarshal(dados, &l.dados)
	}
	return l
}

// Atualiza registra um novo ping pra um servidor, combinando com a média
// anterior via EMA, e salva o arquivo inteiro de novo (igual o Cooldown já
// faz: simples, e o arquivo é pequeno mesmo com centenas de servidores).
func (l *LatenciaCatalogo) Atualiza(servidor string, ms int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	atual, existe := l.dados[servidor]
	if !existe {
		l.dados[servidor] = amostraLatencia{Media: float64(ms), Amostras: 1}
	} else {
		atual.Media = pesoAmostraNova*float64(ms) + (1-pesoAmostraNova)*atual.Media
		atual.Amostras++
		l.dados[servidor] = atual
	}

	dados, err := json.Marshal(l.dados)
	if err == nil {
		os.WriteFile(l.caminho, dados, 0o644)
	}
}

// TopN devolve, dentre os candidatos dados, os n com a menor média de ping.
// Servidor sem nenhuma amostra ainda entra por último (depois de todos os
// já medidos), pra nunca devolver menos do que n candidatos só por falta de
// dado — isso importa principalmente logo depois do primeiro boot, antes do
// pinger de fundo completar uma volta inteira no catálogo.
func (l *LatenciaCatalogo) TopN(candidatos []string, n int) []string {
	l.mu.Lock()
	tipo := make([]struct {
		nome   string
		media  float64
		medido bool
	}, len(candidatos))
	for i, nome := range candidatos {
		a, ok := l.dados[nome]
		tipo[i].nome = nome
		tipo[i].medido = ok
		if ok {
			tipo[i].media = a.Media
		}
	}
	l.mu.Unlock()

	sort.SliceStable(tipo, func(i, j int) bool {
		if tipo[i].medido != tipo[j].medido {
			return tipo[i].medido // medido vem antes de não medido
		}
		return tipo[i].media < tipo[j].media
	})

	if n > len(tipo) {
		n = len(tipo)
	}
	resultado := make([]string, n)
	for i := 0; i < n; i++ {
		resultado[i] = tipo[i].nome
	}
	return resultado
}

// RepingaCatalogo testa (em paralelo, com um teto de concorrência pra não
// disparar centenas de pings de uma vez) o Endpoint de cada servidor do
// catálogo e atualiza a média de cada um. Servidor com .conf ilegível, sem
// resolver, ou que não responde a tempo só fica de fora dessa rodada — não
// trava as outras nem derruba o processo.
func (l *LatenciaCatalogo) RepingaCatalogo(dir string, catalogo []string) {
	sem := make(chan struct{}, 20)
	var wg sync.WaitGroup
	for _, nome := range catalogo {
		wg.Add(1)
		go func(nome string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			host, err := lerEndpointHost(confPath(dir, nome))
			if err != nil {
				log.Print(T("log_latency_check_failed", nome, err))
				return
			}
			ms, err := pingMs(host, pingTimeout)
			if err != nil {
				log.Print(T("log_latency_check_failed", nome, err))
				return
			}
			l.Atualiza(nome, ms)
		}(nome)
	}
	wg.Wait()
}
