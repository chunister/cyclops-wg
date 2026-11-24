package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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
