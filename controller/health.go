package main

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
)

// Um teste de saúde mínimo: abre uma conexão SOCKS5 sem autenticação até o
// microsocks do gateway (porta 1090) e pede um CONNECT até alvo. Se completar,
// o túnel está de pé. Mesma ideia do healthcheck.sh do gateway, vista do controlador.

const (
	socks5Versao   = 0x05
	semAutenticar  = 0x00
	comandoConnect = 0x01
	tipoDominio    = 0x03
)

func testaGateway(enderecoGateway, alvo string, timeout time.Duration) error {
	c, err := net.DialTimeout("tcp", enderecoGateway, timeout)
	if err != nil {
		return err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(timeout))

	if _, err := c.Write([]byte{socks5Versao, 1, semAutenticar}); err != nil {
		return err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(c, resp); err != nil {
		return err
	}
	if resp[0] != socks5Versao || resp[1] != semAutenticar {
		return errors.New("gateway recusou conexão sem autenticação")
	}

	host, portaStr, err := net.SplitHostPort(alvo)
	if err != nil {
		return err
	}
	porta, err := parsePorta(portaStr)
	if err != nil {
		return err
	}

	pedido := []byte{socks5Versao, comandoConnect, 0x00, tipoDominio, byte(len(host))}
	pedido = append(pedido, []byte(host)...)
	portaBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portaBytes, porta)
	pedido = append(pedido, portaBytes...)

	if _, err := c.Write(pedido); err != nil {
		return err
	}

	cabecalho := make([]byte, 4)
	if _, err := io.ReadFull(c, cabecalho); err != nil {
		return err
	}
	if cabecalho[1] != 0x00 {
		return errors.New("gateway recusou o CONNECT: túnel provavelmente sem handshake")
	}
	return nil
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
