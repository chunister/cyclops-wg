package main

import (
	"os"
	"testing"
	"time"
)

func TestEscolheServidoresDoRepoReal(t *testing.T) {
	catalogo, err := Catalogo("../servers")
	if err != nil {
		t.Fatalf("catálogo: %v", err)
	}
	if len(catalogo) < 13 {
		t.Fatalf("catálogo tem só %d servidores, esperava pelo menos 13", len(catalogo))
	}

	tmp, err := os.CreateTemp("", "cooldown-*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	cd := AbreCooldown(tmp.Name())

	usados := map[string]bool{}
	var escolhidos []string
	for i := 0; i < 13; i++ {
		s, err := Escolhe(catalogo, usados, cd, 600*time.Second)
		if err != nil {
			t.Fatalf("escolhe #%d: %v", i, err)
		}
		usados[s] = true
		escolhidos = append(escolhidos, s)
	}

	vistos := map[string]bool{}
	for _, s := range escolhidos {
		if vistos[s] {
			t.Fatalf("servidor repetido: %s", s)
		}
		vistos[s] = true
	}

	if !cd.EmEspera(escolhidos[0], 600*time.Second) {
		t.Fatal("servidor recém-escolhido deveria estar em cooldown")
	}

	// A persistência em disco precisa sobreviver a reabrir o arquivo.
	cd2 := AbreCooldown(tmp.Name())
	if !cd2.EmEspera(escolhidos[0], 600*time.Second) {
		t.Fatal("cooldown não persistiu no arquivo")
	}
}

func TestEscolheFalhaSemServidorLivre(t *testing.T) {
	catalogo := []string{"a", "b"}
	tmp, _ := os.CreateTemp("", "cooldown-*.json")
	tmp.Close()
	defer os.Remove(tmp.Name())
	cd := AbreCooldown(tmp.Name())

	usados := map[string]bool{"a": true, "b": true}
	if _, err := Escolhe(catalogo, usados, cd, time.Second); err == nil {
		t.Fatal("deveria falhar: nenhum servidor livre")
	}
}
