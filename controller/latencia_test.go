package main

import (
	"os"
	"testing"
)

func TestLatenciaTopNOrdenaPeloMenorPing(t *testing.T) {
	tmp, _ := os.CreateTemp("", "latencia-*.json")
	tmp.Close()
	defer os.Remove(tmp.Name())

	l := AbreLatencia(tmp.Name())
	l.Atualiza("lento", 300)
	l.Atualiza("rapido", 20)
	l.Atualiza("medio", 100)
	// "nunca-medido" fica de fora do mapa de propósito.

	top := l.TopN([]string{"lento", "rapido", "medio", "nunca-medido"}, 2)
	if len(top) != 2 {
		t.Fatalf("esperava 2 resultados, veio %d: %v", len(top), top)
	}
	if top[0] != "rapido" || top[1] != "medio" {
		t.Fatalf("esperava [rapido medio], veio %v", top)
	}
}

func TestLatenciaTopNPreenchemComNaoMedidosSeFaltar(t *testing.T) {
	tmp, _ := os.CreateTemp("", "latencia-*.json")
	tmp.Close()
	defer os.Remove(tmp.Name())

	l := AbreLatencia(tmp.Name())
	l.Atualiza("rapido", 20)

	// Pede mais candidatos do que tem medido: o resto deve vir dos não
	// medidos, não truncar a lista por falta de dado.
	top := l.TopN([]string{"rapido", "sem-dado-a", "sem-dado-b"}, 3)
	if len(top) != 3 {
		t.Fatalf("esperava 3 resultados mesmo faltando dado, veio %d: %v", len(top), top)
	}
	if top[0] != "rapido" {
		t.Fatalf("o medido deveria vir primeiro, veio %v", top)
	}
}

func TestLatenciaAtualizaPersiste(t *testing.T) {
	tmp, _ := os.CreateTemp("", "latencia-*.json")
	tmp.Close()
	defer os.Remove(tmp.Name())

	l := AbreLatencia(tmp.Name())
	l.Atualiza("servidor", 50)
	l.Atualiza("servidor", 150)

	l2 := AbreLatencia(tmp.Name())
	top := l2.TopN([]string{"servidor"}, 1)
	if len(top) != 1 || top[0] != "servidor" {
		t.Fatalf("média não persistiu como esperado: %v", top)
	}
}
