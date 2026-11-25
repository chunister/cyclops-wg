// cyclops-wg balancer: um ponto de entrada SOCKS5 que distribui as conexões
// entre os gateways WireGuard, pulando os que estiverem fora do ar.
package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	enderecos := dividePorVirgula(env("GATEWAYS", ""))
	if len(enderecos) == 0 {
		log.Print("GATEWAYS vazio no arranque, esperando um PUT /gateways")
	}

	pool := NovoPool(enderecos)

	intervalo := time.Duration(envInt("HEALTH_INTERVAL", 15)) * time.Second
	limiteFalhas := envInt("HEALTH_FAILS", 3)
	alvoVerificacao := env("HEALTH_TARGET", "1.1.1.1:443")
	timeoutConexao := time.Duration(envInt("DIAL_TIMEOUT", 10)) * time.Second

	go rodaVerificacoes(pool, intervalo, limiteFalhas, alvoVerificacao, timeoutConexao)

	historico := NovoHistorico(envInt("CONN_HISTORY", 20))
	metricas := NovoMetricas()

	if porta := env("API_PORT", ""); porta != "" {
		serveStatus(pool, historico, metricas, ":"+porta)
	}

	escuta := ":" + env("SOCKS_PORT", "1080")
	ln, err := net.Listen("tcp", escuta)
	if err != nil {
		log.Fatalf("não consegui escutar em %s: %v", escuta, err)
	}
	log.Printf("balanceador SOCKS5 em %s, %d gateway(s)", escuta, len(enderecos))

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("erro ao aceitar conexão: %v", err)
			continue
		}
		go atendeCliente(conn, pool, timeoutConexao, historico, metricas)
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

func dividePorVirgula(s string) []string {
	if s == "" {
		return nil
	}
	partes := strings.Split(s, ",")
	var resultado []string
	for _, p := range partes {
		p = strings.TrimSpace(p)
		if p != "" {
			resultado = append(resultado, p)
		}
	}
	return resultado
}
