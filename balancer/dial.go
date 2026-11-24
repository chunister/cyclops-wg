package main

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
)

// conectaViaSocks5 abre uma conexão TCP com enderecoGateway e pede a ele, em
// SOCKS5 sem autenticação, um CONNECT até destino. É o mesmo protocolo de
// socks5.go, agora do lado cliente, falando com o microsocks de um gateway.
func conectaViaSocks5(enderecoGateway, destino string, timeout time.Duration) (net.Conn, error) {
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

	host, portaStr, err := net.SplitHostPort(destino)
	if err != nil {
		c.Close()
		return nil, err
	}
	var porta uint16
	if _, err := fmtSscanPorta(portaStr, &porta); err != nil {
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

	// Resposta: VER, REP, RSV, ATYP, depois o endereço (tamanho varia conforme ATYP) e a porta.
	cabecalho := make([]byte, 4)
	if _, err := io.ReadFull(c, cabecalho); err != nil {
		c.Close()
		return nil, err
	}
	if cabecalho[1] != 0x00 {
		c.Close()
		return nil, errors.New("gateway recusou o CONNECT")
	}

	var tamanhoEndereco int
	switch cabecalho[3] {
	case tipoIPv4:
		tamanhoEndereco = 4
	case tipoIPv6:
		tamanhoEndereco = 16
	case tipoDominio:
		tam := make([]byte, 1)
		if _, err := io.ReadFull(c, tam); err != nil {
			c.Close()
			return nil, err
		}
		tamanhoEndereco = int(tam[0])
	default:
		c.Close()
		return nil, errors.New("tipo de endereço desconhecido na resposta")
	}
	if _, err := io.CopyN(io.Discard, c, int64(tamanhoEndereco+2)); err != nil {
		c.Close()
		return nil, err
	}

	c.SetDeadline(time.Time{})
	return c, nil
}

// fmtSscanPorta evita importar fmt só para isso.
func fmtSscanPorta(s string, porta *uint16) (int, error) {
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
	*porta = uint16(v)
	return 1, nil
}
