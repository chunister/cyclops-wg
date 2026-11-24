package main

import (
	"log"
	"time"
)

// rodaVerificacoes testa cada gateway a cada intervalo, em paralelo, tentando
// um CONNECT real até alvo. Isso é o mesmo tipo de checagem do healthcheck.sh
// do gateway, mas decide a rotação do balanceador, não o estado do Docker.
func rodaVerificacoes(pool *Pool, intervalo time.Duration, limiteFalhas int, alvo string, timeout time.Duration) {
	ticker := time.NewTicker(intervalo)
	defer ticker.Stop()

	for range ticker.C {
		for _, g := range pool.Todos() {
			go verifica(g, alvo, timeout, limiteFalhas)
		}
	}
}

func verifica(g *Gateway, alvo string, timeout time.Duration, limiteFalhas int) {
	conn, err := conectaViaSocks5(g.Addr, alvo, timeout)
	if err != nil {
		g.marcaFalha(limiteFalhas)
		log.Printf("verificação falhou em %s: %v", g.Addr, err)
		return
	}
	conn.Close()
	g.marcaOk()
}
