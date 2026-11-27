package main

import (
	"context"
	"log"
	"net"
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

// enderecosLocaisIPv4 lista os IPv4 das interfaces de rede do host, sem o
// loopback. Usada tanto pra montar a lista "rede interna" no dashboard
// quanto pra validar SOCKS_BIND_IP contra as interfaces reais da máquina.
func enderecosLocaisIPv4() []string {
	var ips []string
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, addr := range ifaces {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			ips = append(ips, ip4.String())
		}
	}
	return ips
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
