package main

import (
	"os"
	"testing"
)

func TestPreferenciasSalvaEcarrega(t *testing.T) {
	tmp, _ := os.CreateTemp("", "prefs-*.json")
	tmp.Close()
	defer os.Remove(tmp.Name())

	preferenciasDashboard{Idioma: "pt-br", Tema: "Dracula"}.salva(tmp.Name())

	p := carregaPreferencias(tmp.Name())
	if p.Idioma != "pt-br" || p.Tema != "Dracula" {
		t.Fatalf("preferências não voltaram como esperado: %+v", p)
	}
}

func TestPreferenciasArquivoInexistenteDevolveZerado(t *testing.T) {
	p := carregaPreferencias("/tmp/cyclops-wg-prefs-que-nao-existe.json")
	if p.Idioma != "" || p.Tema != "" {
		t.Fatalf("esperava struct zerada pra arquivo inexistente, veio %+v", p)
	}
}

func TestIndiceDoTemaNaoEncontrado(t *testing.T) {
	if _, ok := indiceDoTema("tema-que-nao-existe"); ok {
		t.Fatal("esperava ok=false pra um nome de tema desconhecido")
	}
	if indice, ok := indiceDoTema(temas[0].Nome); !ok || indice != 0 {
		t.Fatalf("esperava encontrar o tema 0 pelo nome, veio indice=%d ok=%v", indice, ok)
	}
}

func TestIdiomaParaStringEIdiomaDeStringSaoConsistentes(t *testing.T) {
	for _, i := range []idioma{idiomaEN, idiomaPT, idiomaES} {
		s := idiomaParaString(i)
		if idiomaDeString(s) != i {
			t.Fatalf("idioma %v virou %q e voltou como %v", i, s, idiomaDeString(s))
		}
	}
}
