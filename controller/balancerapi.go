package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Conexao espelha o que o balanceador devolve em GET /conexoes: só para
// mostrar no dashboard, de onde veio o cliente e por qual gateway ele saiu.
type Conexao struct {
	Quando  time.Time `json:"quando"`
	Origem  string    `json:"origem"`
	Destino string    `json:"destino"`
	Gateway string    `json:"gateway"`
}

// ConexoesRecentes busca as últimas conexões vistas pelo balanceador.
func ConexoesRecentes(enderecoAPI string) ([]Conexao, error) {
	cliente := &http.Client{Timeout: 5 * time.Second}
	resp, err := cliente.Get(enderecoAPI + "/conexoes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("balanceador respondeu %d", resp.StatusCode)
	}

	var conexoes []Conexao
	if err := json.NewDecoder(resp.Body).Decode(&conexoes); err != nil {
		return nil, err
	}
	return conexoes, nil
}

// Throughput busca os totais acumulados de upload/download do balanceador.
// Quem chama calcula a taxa comparando duas leituras no tempo, como o
// dashboard já faz com a amostra de CPU do host.
func Throughput(enderecoAPI string) (upload, download int64, err error) {
	cliente := &http.Client{Timeout: 5 * time.Second}
	resp, err := cliente.Get(enderecoAPI + "/throughput")
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("balanceador respondeu %d", resp.StatusCode)
	}

	var r struct {
		UploadBytes   int64 `json:"upload_bytes"`
		DownloadBytes int64 `json:"download_bytes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return 0, 0, err
	}
	return r.UploadBytes, r.DownloadBytes, nil
}

// AtualizaBalanceador manda a lista atual de gateways ativos para o
// endpoint PUT /gateways do balanceador, sem precisar reiniciá-lo.
func AtualizaBalanceador(enderecoAPI string, enderecos []string) error {
	corpo, err := json.Marshal(struct {
		Enderecos []string `json:"enderecos"`
	}{Enderecos: enderecos})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, enderecoAPI+"/gateways", bytes.NewReader(corpo))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	cliente := &http.Client{Timeout: 5 * time.Second}
	resp, err := cliente.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("balanceador respondeu %d", resp.StatusCode)
	}
	return nil
}
