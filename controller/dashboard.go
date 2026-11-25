package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Dashboard mostra o estado do controlador no terminal, ao vivo, no lugar
// do log cru. É montado só quando DASHBOARD está ligado.
type Dashboard struct {
	app     *tview.Application
	resumo  *tview.TextView
	tabela  *tview.Table
	eventos *tview.TextView
}

func NovoDashboard() *Dashboard {
	d := &Dashboard{
		app:     tview.NewApplication(),
		resumo:  tview.NewTextView().SetDynamicColors(true),
		tabela:  tview.NewTable().SetBorders(false),
		eventos: tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetMaxLines(500),
	}
	d.eventos.SetChangedFunc(func() { d.eventos.ScrollToEnd() })

	d.resumo.SetBorder(true).SetTitle(" cyclops-wg ")
	d.tabela.SetBorder(true).SetTitle(" gateways ")
	d.eventos.SetBorder(true).SetTitle(" eventos (q ou ctrl-c pra sair) ")

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.resumo, 3, 0, false).
		AddItem(d.tabela, 0, 2, false).
		AddItem(d.eventos, 0, 1, false)

	d.app.SetRoot(layout, true)
	d.app.SetInputCapture(func(evento *tcell.EventKey) *tcell.EventKey {
		if evento.Rune() == 'q' || evento.Key() == tcell.KeyCtrlC {
			d.app.Stop()
			return nil
		}
		return evento
	})

	return d
}

// Write faz o Dashboard servir de saída para o log padrão: cada linha
// logada vira uma linha no painel de eventos, em vez de ir pro stdout cru.
func (d *Dashboard) Write(p []byte) (int, error) {
	linha := strings.TrimRight(string(p), "\n")
	d.app.QueueUpdateDraw(func() {
		fmt.Fprintln(d.eventos, tview.Escape(linha))
	})
	return len(p), nil
}

// Atualiza redesenha o resumo e a tabela a partir de uma cópia do estado
// atual dos slots, tirada com o mutex do controlador já liberado.
func (d *Dashboard) Atualiza(inicio time.Time, slots []*Slot) {
	d.app.QueueUpdateDraw(func() {
		ativosOk, ativosTotal, reservaOk, reservaTotal := 0, 0, 0, 0
		for _, s := range slots {
			if s.Papel == papelAtivo {
				ativosTotal++
				if s.Falhas == 0 {
					ativosOk++
				}
			} else {
				reservaTotal++
				if s.Falhas == 0 && !s.EmReparo {
					reservaOk++
				}
			}
		}
		uptime := time.Since(inicio).Round(time.Second)
		d.resumo.SetText(fmt.Sprintf(
			"ativos saudáveis: [green]%d[white]/%d    reservas saudáveis: [green]%d[white]/%d    no ar há %s",
			ativosOk, ativosTotal, reservaOk, reservaTotal, uptime,
		))

		d.tabela.Clear()
		cabecalho := []string{"GATEWAY", "PAPEL", "SERVIDOR", "IP", "FALHAS", "ESTADO"}
		for col, titulo := range cabecalho {
			d.tabela.SetCell(0, col, tview.NewTableCell(titulo).
				SetTextColor(tcell.ColorYellow).
				SetSelectable(false))
		}

		ordenados := append([]*Slot{}, slots...)
		sort.Slice(ordenados, func(i, j int) bool {
			if ordenados[i].Papel != ordenados[j].Papel {
				return ordenados[i].Papel == papelAtivo
			}
			return ordenados[i].Nome < ordenados[j].Nome
		})

		for i, s := range ordenados {
			estado, cor := "saudável", tcell.ColorGreen
			switch {
			case s.EmReparo:
				estado, cor = "reparando", tcell.ColorYellow
			case s.Falhas > 0:
				estado, cor = "falhando", tcell.ColorRed
			}
			linha := i + 1
			d.tabela.SetCell(linha, 0, tview.NewTableCell(s.Nome))
			d.tabela.SetCell(linha, 1, tview.NewTableCell(s.Papel))
			d.tabela.SetCell(linha, 2, tview.NewTableCell(s.Servidor))
			d.tabela.SetCell(linha, 3, tview.NewTableCell(s.IP))
			d.tabela.SetCell(linha, 4, tview.NewTableCell(fmt.Sprintf("%d", s.Falhas)))
			d.tabela.SetCell(linha, 5, tview.NewTableCell(estado).SetTextColor(cor))
		}
	})
}

// Executa bloqueia rodando o loop de interface até o usuário sair (q ou ctrl-c).
func (d *Dashboard) Executa() error {
	return d.app.Run()
}
