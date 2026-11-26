package main

import (
	"encoding/json"
	"os"
)

// preferenciasDashboard é o que fica salvo em PREFS_FILE entre execuções:
// o idioma e o tema escolhidos pela última vez pelos atalhos do dashboard
// (l/L, t/T), pra abrir no mesmo estado da próxima vez, em vez de sempre
// voltar pro DASH_LANG/tema padrão do .env.
type preferenciasDashboard struct {
	Idioma string `json:"idioma,omitempty"`
	Tema   string `json:"tema,omitempty"`
}

// carregaPreferencias lê o arquivo se existir; se não existir ainda (nunca
// trocou nada, ou é a primeira execução), devolve a struct zerada, e quem
// chama simplesmente mantém o default de cada coisa.
func carregaPreferencias(caminho string) preferenciasDashboard {
	var p preferenciasDashboard
	if caminho == "" {
		return p
	}
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return p
	}
	json.Unmarshal(dados, &p)
	return p
}

func (p preferenciasDashboard) salva(caminho string) {
	dados, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(caminho, dados, 0o644)
}

// idiomaParaString devolve o mesmo formato aceito por DASH_LANG/idiomaDeString,
// pra o arquivo de preferências e a variável de ambiente falarem a mesma língua.
func idiomaParaString(i idioma) string {
	switch i {
	case idiomaPT:
		return "pt-br"
	case idiomaES:
		return "es"
	default:
		return "en"
	}
}

// indiceDoTema procura um tema pelo nome salvo. Devolve ok=false se o nome
// não bater com nenhum tema atual (por exemplo, um arquivo de preferências
// de uma versão antiga com um tema que não existe mais) — quem chama
// simplesmente ignora e mantém o tema padrão.
func indiceDoTema(nome string) (int, bool) {
	for i, t := range temas {
		if t.Nome == nome {
			return i, true
		}
	}
	return 0, false
}
