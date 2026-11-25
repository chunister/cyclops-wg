package main

import "sync/atomic"

// Metricas acumula os bytes que passam pelo proxy desde que o balanceador
// subiu. Quem usa isso (o controlador, via GET /throughput) calcula a taxa
// comparando duas leituras no tempo, do mesmo jeito que o dashboard já faz
// com a amostra de CPU do host.
type Metricas struct {
	bytesUpload   atomic.Int64 // cliente -> gateway -> destino
	bytesDownload atomic.Int64 // destino -> gateway -> cliente
}

func NovoMetricas() *Metricas {
	return &Metricas{}
}

func (m *Metricas) Totais() (upload, download int64) {
	return m.bytesUpload.Load(), m.bytesDownload.Load()
}
