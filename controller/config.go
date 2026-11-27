package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	GatewayCount         int
	SpareCount           int
	WGPrivateKey         string
	WGAddress            string
	ServersDir           string
	StateFile            string
	RepoRoot             string
	NetworkName          string
	GatewayImage         string
	BalancerImage        string
	BalancerAddr         string // host:porta do próprio balanceador, visto pelo controlador
	HealthInterval       int    // segundos
	HealthFails          int
	ServerCooldown       int // segundos
	SocksPort            string
	SocksBindIP          string // "127.0.0.1" (só local), "0.0.0.0" (todas as interfaces), ou uma IP de rede específica do host
	ApiPort              string
	ControllerPort       string
	Dashboard            bool
	FullRotateInterval   int // segundos; 0 desliga a rotação periódica completa
	Lang                 string
	HealthTarget         string // alvo usado pelo balanceador pra testar cada gateway
	DialTimeout          int    // segundos; tempo máximo de uma conexão de teste do balanceador
	ConnHistory          int    // quantas conexões recentes o balanceador guarda em memória
	LogFile              string // "" desliga; além da tela, grava o log também num arquivo
	LatencyTopServers    int    // 0 desliga. Sorteia só entre os N servidores com menor ping médio.
	LatencyCheckInterval int    // segundos entre re-pings do catálogo inteiro
	LatencyFile          string
	PrefsFile            string // "" desliga: idioma/tema do dashboard não são salvos entre execuções
}

// diretorioExecutavel devolve a pasta onde o binário compilado do
// controlador está (via /proc/self/exe), não o diretório de onde ele foi
// chamado (esse é o pwd, e é exatamente o que causa o problema: muda
// conforme de onde você roda "./controller"). É a partir daqui, não do pwd,
// que os caminhos relativos abaixo são ancorados, pra funcionar igual não
// importa se você chama de dentro de controller/, por um caminho absoluto
// de outro lugar, ou via systemd com outro WorkingDirectory. Se não der pra
// descobrir (bem raro), cai pra "." e volta a depender do pwd, só como
// último recurso.
func diretorioExecutavel() string {
	caminho, err := os.Executable()
	if err != nil {
		return "."
	}
	if resolvido, err := filepath.EvalSymlinks(caminho); err == nil {
		caminho = resolvido
	}
	return filepath.Dir(caminho)
}

// resolveRelativo ancora um caminho relativo (seja o default ou um valor
// vindo do .env) na pasta do executável. Um caminho já absoluto passa
// direto, sem mudança.
func resolveRelativo(base, caminho string) string {
	if caminho == "" || filepath.IsAbs(caminho) {
		return caminho
	}
	return filepath.Join(base, caminho)
}

func carregaConfig() Config {
	exeDir := diretorioExecutavel()
	raiz := resolveRelativo(exeDir, env("REPO_ROOT", ".."))
	return Config{
		GatewayCount:         envInt("GATEWAY_COUNT", 10),
		SpareCount:           envInt("SPARE_COUNT", 3),
		WGPrivateKey:         env("WG_PRIVATE_KEY", ""),
		WGAddress:            env("WG_ADDRESS", "10.14.0.2/16"),
		ServersDir:           resolveRelativo(exeDir, env("SERVERS_DIR", filepath.Join(raiz, "servers"))),
		StateFile:            resolveRelativo(exeDir, env("STATE_FILE", "./controller-state.json")),
		RepoRoot:             raiz,
		NetworkName:          env("DOCKER_NETWORK", "cyclops-net"),
		GatewayImage:         env("GATEWAY_IMAGE", "cyclops-gateway:dev"),
		BalancerImage:        env("BALANCER_IMAGE", "cyclops-balancer:dev"),
		BalancerAddr:         env("BALANCER_API_ADDR", "http://127.0.0.1:8081"),
		HealthInterval:       envInt("HEALTH_INTERVAL", 15),
		HealthFails:          envInt("HEALTH_FAILS", 3),
		ServerCooldown:       envInt("SERVER_COOLDOWN", 600),
		SocksPort:            env("SOCKS_PORT", "1080"),
		SocksBindIP:          validaSocksBindIP(env("SOCKS_BIND_IP", "0.0.0.0")),
		ApiPort:              env("API_PORT", "8081"),
		ControllerPort:       env("CONTROLLER_PORT", "9090"),
		Dashboard:            envBool("DASHBOARD", true),
		FullRotateInterval:   envInt("FULL_ROTATE_INTERVAL", 0),
		Lang:                 env("DASH_LANG", "en"),
		HealthTarget:         env("HEALTH_TARGET", "1.1.1.1:443"),
		DialTimeout:          envInt("DIAL_TIMEOUT", 10),
		ConnHistory:          envInt("CONN_HISTORY", 20),
		LogFile:              resolveRelativo(exeDir, env("LOG_FILE", "./controller.log")),
		LatencyTopServers:    envInt("LATENCY_TOP_SERVERS", 0),
		LatencyCheckInterval: envInt("LATENCY_CHECK_INTERVAL", 600),
		LatencyFile:          resolveRelativo(exeDir, env("LATENCY_FILE", "./latencia.json")),
		PrefsFile:            resolveRelativo(exeDir, env("PREFS_FILE", "./dashboard-prefs.json")),
	}
}

// validaSocksBindIP aceita "0.0.0.0" e "127.0.0.1" direto, e qualquer outro
// valor só se bater com uma IP de alguma interface de rede real do host;
// fora isso, avisa no log e cai pro padrão "0.0.0.0" (todas as interfaces),
// pra não travar o controlador por causa de um IP digitado errado no .env.
func validaSocksBindIP(v string) string {
	if v == "0.0.0.0" || v == "127.0.0.1" {
		return v
	}
	for _, ip := range enderecosLocaisIPv4() {
		if ip == v {
			return v
		}
	}
	log.Print(T("log_invalid_bind_ip", v))
	return "0.0.0.0"
}

func env(chave, padrao string) string {
	if v := os.Getenv(chave); v != "" {
		return v
	}
	return padrao
}

func envInt(chave string, padrao int) int {
	v := os.Getenv(chave)
	if v == "" {
		return padrao
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Print(T("log_invalid_env_int", chave, v, padrao))
		return padrao
	}
	return n
}

func envBool(chave string, padrao bool) bool {
	v := os.Getenv(chave)
	if v == "" {
		return padrao
	}
	switch strings.ToLower(v) {
	case "0", "false", "nao", "não":
		return false
	case "1", "true", "sim":
		return true
	default:
		log.Print(T("log_invalid_env_bool", chave, v, padrao))
		return padrao
	}
}
