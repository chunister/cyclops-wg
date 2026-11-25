package main

import (
	"sync"
	"time"
)

// Conexao registra uma conexão recente, só para controle visual: de onde
// veio o cliente, para onde ele queria ir, e por qual gateway saiu.
type Conexao struct {
	Quando  time.Time `json:"quando"`
	Origem  string    `json:"origem"`
	Destino string    `json:"destino"`
	Gateway string    `json:"gateway"`
}

// HistoricoConexoes guarda só as últimas N conexões, em memória, sem
// persistir nada em disco.
type HistoricoConexoes struct {
	mu       sync.Mutex
	limite   int
	recentes []Conexao
}

func NovoHistorico(limite int) *HistoricoConexoes {
	if limite <= 0 {
		limite = 20
	}
	return &HistoricoConexoes{limite: limite}
}

func (h *HistoricoConexoes) Registra(origem, destino, gateway string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recentes = append(h.recentes, Conexao{
		Quando:  time.Now(),
		Origem:  origem,
		Destino: destino,
		Gateway: gateway,
	})
	if excesso := len(h.recentes) - h.limite; excesso > 0 {
		h.recentes = h.recentes[excesso:]
	}
}

// Recentes devolve as conexões guardadas, da mais nova para a mais velha.
func (h *HistoricoConexoes) Recentes() []Conexao {
	h.mu.Lock()
	defer h.mu.Unlock()
	copia := make([]Conexao, len(h.recentes))
	copy(copia, h.recentes)
	for i, j := 0, len(copia)-1; i < j; i, j = i+1, j-1 {
		copia[i], copia[j] = copia[j], copia[i]
	}
	return copia
}
