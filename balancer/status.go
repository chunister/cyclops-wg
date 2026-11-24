package main

import (
	"encoding/json"
	"net/http"
)

type statusGateway struct {
	Addr    string `json:"addr"`
	Healthy bool   `json:"healthy"`
}

// serveStatus expõe GET /status com o estado de cada gateway. Serve para
// acompanhar a rotação de fora, sem precisar ler o log.
func serveStatus(pool *Pool, endereco string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		var lista []statusGateway
		for _, g := range pool.Todos() {
			lista = append(lista, statusGateway{Addr: g.Addr, Healthy: g.isHealthy()})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(lista)
	})
	go http.ListenAndServe(endereco, mux)
}
