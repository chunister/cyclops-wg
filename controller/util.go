package main

import (
	"context"
	"log"
	"net"
	"path/filepath"
	"strings"
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

// prefixosInterfaceVirtual nomeia os prefixos de interface que o Linux (e o
// macOS, pros testes locais) usam pra redes criadas por software, não pra
// placas de rede de verdade: pontes e interfaces do Docker ("docker0",
// "br-xxxxxxxx" por rede, "veth" por container), do WireGuard ("wg0"), de
// outras VPNs/containers comuns (libvirt, Podman, k8s, Tailscale, ZeroTier),
// e os equivalentes do macOS. Sem esse filtro, cada rede Docker que o
// cyclops-wg (ou qualquer outro container no host) criar apareceria como
// mais uma "rede interna" na lista do dashboard.
var prefixosInterfaceVirtual = []string{
	"docker", "br-", "veth", "virbr", "vmnet", "vboxnet",
	"tun", "tap", "wg", "utun", "podman", "cni", "flannel",
	"cali", "zt", "tailscale", "anbox", "awdl", "llw", "bridge",
}

func interfaceEhVirtual(nome string) bool {
	nome = strings.ToLower(nome)
	for _, prefixo := range prefixosInterfaceVirtual {
		if strings.HasPrefix(nome, prefixo) {
			return true
		}
	}
	return false
}

// enderecosLocaisIPv4 lista os IPv4 das interfaces de rede "de verdade" do
// host (ignora loopback e as interfaces virtuais de prefixosInterfaceVirtual
// acima, como as pontes que o Docker cria por trás de cada rede). Usada
// tanto pra montar a lista "rede interna" no dashboard quanto pra validar
// SOCKS_BIND_IP contra as interfaces reais da máquina.
func enderecosLocaisIPv4() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if interfaceEhVirtual(iface.Name) {
			continue
		}
		enderecos, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range enderecos {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				ips = append(ips, ip4.String())
			}
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
