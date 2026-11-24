package main

import (
	"context"
	"log"
	"path/filepath"
	"time"
)

func logPrintf(formato string, args ...interface{}) {
	log.Printf(formato, args...)
}

func absPath(caminho string) string {
	abs, err := filepath.Abs(caminho)
	if err != nil {
		return caminho
	}
	return abs
}

// esperaContainerRodando espera o container sair do zero até ele aparecer
// como rodando, depois de um restart, antes de tentar um exec nele.
func esperaContainerRodando(ctx context.Context, d *Docker, nome string, prazo time.Duration) bool {
	limite := time.Now().Add(prazo)
	for time.Now().Before(limite) {
		if rodando, err := d.EstaRodando(ctx, nome); err == nil && rodando {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}
