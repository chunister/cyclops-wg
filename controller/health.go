package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// Um teste de saúde mínimo: abre uma conexão SOCKS5 sem autenticação até o
// microsocks do gateway (porta 1090) e pede um CONNECT até alvo. Se completar,
// o túnel está de pé. Mesma ideia do healthcheck.sh do gateway, vista do controlador.

const (
	socks5Versao   = 0x05
	semAutenticar  = 0x00
	comandoConnect = 0x01
	tipoIPv4       = 0x01
	tipoDominio    = 0x03
	tipoIPv6       = 0x04
)

// conectaSocks5 faz a saudação e o CONNECT do protocolo SOCKS5 (RFC 1928)
// contra o microsocks de um gateway, e devolve a conexão já pronta para uso,
// com a resposta do CONNECT inteiramente consumida.
func conectaSocks5(enderecoGateway, alvo string, timeout time.Duration) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", enderecoGateway, timeout)
	if err != nil {
		return nil, err
	}
	c.SetDeadline(time.Now().Add(timeout))

	if _, err := c.Write([]byte{socks5Versao, 1, semAutenticar}); err != nil {
		c.Close()
		return nil, err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(c, resp); err != nil {
		c.Close()
		return nil, err
	}
	if resp[0] != socks5Versao || resp[1] != semAutenticar {
		c.Close()
		return nil, errors.New("gateway recusou conexão sem autenticação")
	}

	host, portaStr, err := net.SplitHostPort(alvo)
	if err != nil {
		c.Close()
		return nil, err
	}
	porta, err := parsePorta(portaStr)
	if err != nil {
		c.Close()
		return nil, err
	}

	pedido := []byte{socks5Versao, comandoConnect, 0x00, tipoDominio, byte(len(host))}
	pedido = append(pedido, []byte(host)...)
	portaBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portaBytes, porta)
	pedido = append(pedido, portaBytes...)

	if _, err := c.Write(pedido); err != nil {
		c.Close()
		return nil, err
	}

	cabecalho := make([]byte, 4)
	if _, err := io.ReadFull(c, cabecalho); err != nil {
		c.Close()
		return nil, err
	}
	if cabecalho[1] != 0x00 {
		c.Close()
		return nil, errors.New("gateway recusou o CONNECT: túnel provavelmente sem handshake")
	}
	if err := descartaEnderecoLigado(c, cabecalho[3]); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// descartaEnderecoLigado consome o BND.ADDR e o BND.PORT que vêm depois do
// cabeçalho da resposta do CONNECT, pra deixar a conexão limpa pro tráfego
// de verdade que vem a seguir.
func descartaEnderecoLigado(c net.Conn, atyp byte) error {
	var tamanho int
	switch atyp {
	case tipoIPv4:
		tamanho = 4 + 2
	case tipoIPv6:
		tamanho = 16 + 2
	case tipoDominio:
		tamanhoHost := make([]byte, 1)
		if _, err := io.ReadFull(c, tamanhoHost); err != nil {
			return err
		}
		tamanho = int(tamanhoHost[0]) + 2
	default:
		return errors.New("ATYP desconhecido na resposta do SOCKS5")
	}
	_, err := io.CopyN(io.Discard, c, int64(tamanho))
	return err
}

func testaGateway(enderecoGateway, alvo string, timeout time.Duration) error {
	c, err := conectaSocks5(enderecoGateway, alvo, timeout)
	if err != nil {
		return err
	}
	c.Close()
	return nil
}

// buscaIPSaida descobre o IP público que o gateway usa pra sair à internet,
// fazendo uma requisição HTTP de verdade pelo próprio SOCKS5 dele até um
// serviço de eco de IP. É o jeito mais direto de mostrar o IP do WireGuard
// no dashboard, sem precisar de acesso direto ao container.
func buscaIPSaida(enderecoGateway string, timeout time.Duration) (string, error) {
	c, err := conectaSocks5(enderecoGateway, "ip-api.com:80", timeout)
	if err != nil {
		return "", err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(timeout))

	pedido := "GET /line/?fields=query HTTP/1.1\r\nHost: ip-api.com\r\nConnection: close\r\n\r\n"
	if _, err := c.Write([]byte(pedido)); err != nil {
		return "", err
	}

	resposta, _ := io.ReadAll(c)
	partes := strings.SplitN(string(resposta), "\r\n\r\n", 2)
	if len(partes) != 2 {
		return "", errors.New("resposta HTTP sem corpo")
	}
	ip := strings.TrimSpace(partes[1])
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("resposta inesperada do serviço de IP: %q", ip)
	}
	return ip, nil
}

func parsePorta(s string) (uint16, error) {
	var v int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("porta inválida")
		}
		v = v*10 + int(r-'0')
	}
	if v < 0 || v > 65535 {
		return 0, errors.New("porta fora de faixa")
	}
	return uint16(v), nil
}
