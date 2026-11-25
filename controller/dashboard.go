package main

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// quantasAmostrasDeCPU é quanto histórico de uso de CPU o sparkline guarda.
// Em amostras de 1s (o intervalo do ticker que chama AtualizaRecursos), são
// os últimos 60s de uso.
const quantasAmostrasDeCPU = 60

// Dashboard mostra o estado do controlador no terminal, ao vivo, no lugar
// do log cru. É montado só quando DASHBOARD está ligado.
type Dashboard struct {
	app      *tview.Application
	resumo   *tview.TextView
	recursos *tview.TextView
	tabela   *tview.Table
	conexoes *tview.Table
	eventos  *tview.TextView

	cpuAnterior    cpuAmostra
	cpuTemAnterior bool
	cpuHistorico   []float64
}

func NovoDashboard() *Dashboard {
	d := &Dashboard{
		app:      tview.NewApplication(),
		resumo:   tview.NewTextView().SetDynamicColors(true),
		recursos: tview.NewTextView().SetDynamicColors(true),
		tabela:   tview.NewTable().SetBorders(false),
		conexoes: tview.NewTable().SetBorders(false),
		eventos:  tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetMaxLines(500),
	}
	d.eventos.SetChangedFunc(func() { d.eventos.ScrollToEnd() })

	d.resumo.SetBorder(true).SetTitle(" cyclops-wg ")
	d.recursos.SetBorder(true).SetTitle(" recursos (host) ")
	d.tabela.SetBorder(true).SetTitle(" gateways ")
	d.conexoes.SetBorder(true).SetTitle(" conexões recentes ")
	d.eventos.SetBorder(true).SetTitle(" eventos (q ou ctrl-c pra sair) ")

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.resumo, 3, 0, false).
		AddItem(d.recursos, 4, 0, false).
		AddItem(d.tabela, 0, 2, false).
		AddItem(d.conexoes, 0, 2, false).
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
		cabecalho := []string{"GATEWAY", "PAPEL", "SERVIDOR", "IP INTERNO", "IP SAÍDA", "FALHAS", "ESTADO"}
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
			ipSaida := s.IPSaida
			if ipSaida == "" {
				ipSaida = "..."
			}
			d.tabela.SetCell(linha, 4, tview.NewTableCell(ipSaida).SetTextColor(tcell.ColorAqua))
			d.tabela.SetCell(linha, 5, tview.NewTableCell(fmt.Sprintf("%d", s.Falhas)))
			d.tabela.SetCell(linha, 6, tview.NewTableCell(estado).SetTextColor(cor))
		}
	})
}

// AtualizaConexoes redesenha a tabela de conexões recentes, mais nova primeiro.
// ipSaida mapeia o IP interno de cada gateway (o que aparece em c.Gateway,
// antes dos dois-pontos da porta) pro IP público do túnel WireGuard dele.
func (d *Dashboard) AtualizaConexoes(conexoes []Conexao, ipSaida map[string]string) {
	d.app.QueueUpdateDraw(func() {
		d.conexoes.Clear()
		cabecalho := []string{"HORA", "ORIGEM", "DESTINO", "GATEWAY", "IP SAÍDA"}
		for col, titulo := range cabecalho {
			d.conexoes.SetCell(0, col, tview.NewTableCell(titulo).
				SetTextColor(tcell.ColorYellow).
				SetSelectable(false))
		}
		for i, c := range conexoes {
			linha := i + 1
			d.conexoes.SetCell(linha, 0, tview.NewTableCell(c.Quando.Format("15:04:05")).
				SetTextColor(tcell.ColorGray))
			d.conexoes.SetCell(linha, 1, tview.NewTableCell(c.Origem).
				SetTextColor(tcell.ColorWhite))
			d.conexoes.SetCell(linha, 2, tview.NewTableCell(c.Destino).
				SetTextColor(tcell.ColorOrange))
			d.conexoes.SetCell(linha, 3, tview.NewTableCell(c.Gateway).
				SetTextColor(tcell.ColorAqua))

			saida := "..."
			if host, _, err := net.SplitHostPort(c.Gateway); err == nil {
				if ip := ipSaida[host]; ip != "" {
					saida = ip
				}
			}
			d.conexoes.SetCell(linha, 4, tview.NewTableCell(saida).
				SetTextColor(tcell.ColorAqua))
		}
	})
}

// AtualizaRecursos lê o uso de CPU e RAM do host e redesenha o painel de
// recursos, com uma barra de blocos pra cada um e um sparkline do histórico
// de CPU dos últimos quantasAmostrasDeCPU segundos.
func (d *Dashboard) AtualizaRecursos() {
	atual, errCPU := lerCPUAmostra()
	var cpuPct float64
	if errCPU == nil {
		if d.cpuTemAnterior {
			cpuPct = percentualCPU(d.cpuAnterior, atual)
			d.cpuHistorico = append(d.cpuHistorico, cpuPct)
			if len(d.cpuHistorico) > quantasAmostrasDeCPU {
				d.cpuHistorico = d.cpuHistorico[len(d.cpuHistorico)-quantasAmostrasDeCPU:]
			}
		}
		d.cpuAnterior = atual
		d.cpuTemAnterior = true
	}
	ramPct, ramUsado, ramTotal, errRAM := usoRAM()

	d.app.QueueUpdateDraw(func() {
		linhaCPU := fmt.Sprintf("CPU  %3.0f%%  [%s]   %s", cpuPct, barraBlocos(cpuPct, 20), sparkline(d.cpuHistorico))
		if errCPU != nil {
			linhaCPU = fmt.Sprintf("CPU  --%%  (sem leitura: %v)", errCPU)
		}
		linhaRAM := fmt.Sprintf("RAM  %3.0f%%  [%s]   %.1f/%.1f GB", ramPct, barraBlocos(ramPct, 20), ramUsado, ramTotal)
		if errRAM != nil {
			linhaRAM = fmt.Sprintf("RAM  --%%  (sem leitura: %v)", errRAM)
		}
		d.recursos.SetText(linhaCPU + "\n" + linhaRAM)
	})
}

func barraBlocos(percentual float64, largura int) string {
	switch {
	case percentual < 0:
		percentual = 0
	case percentual > 100:
		percentual = 100
	}
	preenchido := int(percentual / 100 * float64(largura))
	return strings.Repeat("█", preenchido) + strings.Repeat("░", largura-preenchido)
}

var nivelSparkline = []rune("▁▂▃▄▅▆▇█")

func sparkline(amostras []float64) string {
	var sb strings.Builder
	for _, v := range amostras {
		switch {
		case v < 0:
			v = 0
		case v > 100:
			v = 100
		}
		idx := int(v / 100 * float64(len(nivelSparkline)-1))
		sb.WriteRune(nivelSparkline[idx])
	}
	return sb.String()
}

// Executa bloqueia rodando o loop de interface até o usuário sair (q ou ctrl-c).
func (d *Dashboard) Executa() error {
	return d.app.Run()
}
