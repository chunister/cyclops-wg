package main

import (
	"encoding/json"
	"io"
	"net/http"
)

type statusGateway struct {
	Addr    string `json:"addr"`
	Healthy bool   `json:"healthy"`
}

// serveStatus expõe GET /status com o estado de cada gateway, e GET
// /conexoes com as últimas conexões vistas. Serve para acompanhar a
// rotação de fora, sem precisar ler o log.
func serveStatus(pool *Pool, historico *HistoricoConexoes, metricas *Metricas, endereco string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		var lista []statusGateway
		for _, g := range pool.Todos() {
			lista = append(lista, statusGateway{Addr: g.Addr, Healthy: g.isHealthy()})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(lista)
	})
	mux.HandleFunc("/conexoes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(historico.Recentes())
	})
	// Totais acumulados de bytes trafegados pelo proxy. Quem lê compara duas
	// chamadas no tempo pra chegar numa taxa (bytes/s), como o dashboard faz.
	mux.HandleFunc("/throughput", func(w http.ResponseWriter, r *http.Request) {
		upload, download := metricas.Totais()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			UploadBytes   int64 `json:"upload_bytes"`
			DownloadBytes int64 `json:"download_bytes"`
		}{upload, download})
	})
	// Só o controlador chama isso, para trocar quem está ativo sem reiniciar o balanceador.
	mux.HandleFunc("/gateways", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		corpo, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var pedido struct {
			Enderecos []string `json:"enderecos"`
		}
		if err := json.Unmarshal(corpo, &pedido); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		pool.DefineGateways(pedido.Enderecos)
		w.WriteHeader(http.StatusNoContent)
	})
	go http.ListenAndServe(endereco, mux)
}
