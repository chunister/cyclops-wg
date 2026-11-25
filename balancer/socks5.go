package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
	"time"
)

// Servidor SOCKS5 mínimo: sem autenticação, só o comando CONNECT (RFC 1928).
// É a parte que fala com o cliente (Burp, curl, etc.). A conexão com o
// gateway escolhido usa o mesmo protocolo, do lado cliente, em dial.go.

const (
	socks5Versao   = 0x05
	semAutenticar  = 0x00
	comandoConnect = 0x01
	tipoIPv4       = 0x01
	tipoDominio    = 0x03
	tipoIPv6       = 0x04
)

func atendeCliente(conexao net.Conn, pool *Pool, timeoutConexao time.Duration, historico *HistoricoConexoes, metricas *Metricas) {
	defer conexao.Close()
	conexao.SetDeadline(time.Now().Add(30 * time.Second))

	if err := saudacao(conexao); err != nil {
		log.Printf("saudação SOCKS5 falhou: %v", err)
		return
	}

	destino, err := leRequisicaoConnect(conexao)
	if err != nil {
		log.Printf("requisição SOCKS5 inválida: %v", err)
		return
	}

	gw := pool.Escolhe()
	if gw == nil {
		responde(conexao, 0x01) // falha geral: nenhum gateway de pé
		log.Printf("sem gateway saudável para %s", destino)
		return
	}

	upstream, err := conectaViaSocks5(gw.Addr, destino, timeoutConexao)
	if err != nil {
		gw.marcaFalha(999) // uma falha de conexão real já tira o gateway da vez seguinte
		responde(conexao, 0x01)
		log.Printf("gateway %s falhou ao conectar em %s: %v", gw.Addr, destino, err)
		return
	}
	defer upstream.Close()

	if err := responde(conexao, 0x00); err != nil {
		return
	}

	if historico != nil {
		historico.Registra(conexao.RemoteAddr().String(), destino, gw.Addr)
	}

	conexao.SetDeadline(time.Time{})
	repassa(conexao, upstream, metricas)
}

func saudacao(c net.Conn) error {
	cabecalho := make([]byte, 2)
	if _, err := io.ReadFull(c, cabecalho); err != nil {
		return err
	}
	if cabecalho[0] != socks5Versao {
		return errors.New("versão não suportada")
	}
	metodos := make([]byte, cabecalho[1])
	if _, err := io.ReadFull(c, metodos); err != nil {
		return err
	}
	_, err := c.Write([]byte{socks5Versao, semAutenticar})
	return err
}

func leRequisicaoConnect(c net.Conn) (string, error) {
	cabecalho := make([]byte, 4)
	if _, err := io.ReadFull(c, cabecalho); err != nil {
		return "", err
	}
	if cabecalho[0] != socks5Versao || cabecalho[1] != comandoConnect {
		return "", errors.New("só o comando CONNECT é suportado")
	}

	var host string
	switch cabecalho[3] {
	case tipoIPv4:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(c, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case tipoIPv6:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(c, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case tipoDominio:
		tam := make([]byte, 1)
		if _, err := io.ReadFull(c, tam); err != nil {
			return "", err
		}
		nome := make([]byte, tam[0])
		if _, err := io.ReadFull(c, nome); err != nil {
			return "", err
		}
		host = string(nome)
	default:
		return "", errors.New("tipo de endereço desconhecido")
	}

	portaBytes := make([]byte, 2)
	if _, err := io.ReadFull(c, portaBytes); err != nil {
		return "", err
	}
	porta := binary.BigEndian.Uint16(portaBytes)

	return fmt.Sprintf("%s:%d", host, porta), nil
}

func responde(c net.Conn, codigo byte) error {
	// BND.ADDR e BND.PORT fixos em 0: o cliente não usa esses campos no modo CONNECT comum.
	msg := []byte{socks5Versao, codigo, 0x00, tipoIPv4, 0, 0, 0, 0, 0, 0}
	_, err := c.Write(msg)
	return err
}

// escritorContador soma no contador cada byte que passa por Write, pra dar
// pra medir a taxa de transferência em tempo real (não só o total quando a
// conexão fecha, já que algumas ficam abertas por minutos).
type escritorContador struct {
	dst      io.Writer
	contador *atomic.Int64
}

func (w *escritorContador) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	if n > 0 {
		w.contador.Add(int64(n))
	}
	return n, err
}

func repassa(cliente, upstream net.Conn, metricas *Metricas) {
	feito := make(chan struct{}, 2)
	copia := func(dst io.Writer, src io.Reader, contador *atomic.Int64) {
		io.Copy(&escritorContador{dst: dst, contador: contador}, src)
		feito <- struct{}{}
	}
	go copia(cliente, upstream, &metricas.bytesDownload) // destino -> gateway -> cliente
	go copia(upstream, cliente, &metricas.bytesUpload)   // cliente -> gateway -> destino
	<-feito
}
