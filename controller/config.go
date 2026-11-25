package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	GatewayCount   int
	SpareCount     int
	WGPrivateKey   string
	WGAddress      string
	ServersDir     string
	StateFile      string
	RepoRoot       string
	NetworkName    string
	GatewayImage   string
	BalancerImage  string
	BalancerAddr   string // host:porta do próprio balanceador, visto pelo controlador
	HealthInterval int    // segundos
	HealthFails    int
	ServerCooldown int // segundos
	SocksPort      string
	ApiPort        string
	ControllerPort string
	Dashboard      bool
}

func carregaConfig() Config {
	raiz := env("REPO_ROOT", "..")
	return Config{
		GatewayCount:   envInt("GATEWAY_COUNT", 10),
		SpareCount:     envInt("SPARE_COUNT", 3),
		WGPrivateKey:   env("WG_PRIVATE_KEY", ""),
		WGAddress:      env("WG_ADDRESS", "10.14.0.2/16"),
		ServersDir:     env("SERVERS_DIR", filepath.Join(raiz, "servers")),
		StateFile:      env("STATE_FILE", "./controller-state.json"),
		RepoRoot:       raiz,
		NetworkName:    env("DOCKER_NETWORK", "cyclops-net"),
		GatewayImage:   env("GATEWAY_IMAGE", "cyclops-gateway:dev"),
		BalancerImage:  env("BALANCER_IMAGE", "cyclops-balancer:dev"),
		BalancerAddr:   env("BALANCER_API_ADDR", "http://127.0.0.1:8081"),
		HealthInterval: envInt("HEALTH_INTERVAL", 15),
		HealthFails:    envInt("HEALTH_FAILS", 3),
		ServerCooldown: envInt("SERVER_COOLDOWN", 600),
		SocksPort:      env("SOCKS_PORT", "1080"),
		ApiPort:        env("API_PORT", "8081"),
		ControllerPort: env("CONTROLLER_PORT", "9090"),
		Dashboard:      envBool("DASHBOARD", true),
	}
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
		log.Printf("valor inválido em %s=%q, usando %d", chave, v, padrao)
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
		log.Printf("valor inválido em %s=%q, usando %v", chave, v, padrao)
		return padrao
	}
}
