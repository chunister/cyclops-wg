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
	app            *tview.Application
	resumo         *tview.TextView
	recursos       *tview.TextView
	tabelaAtivos   *tview.Table
	tabelaReservas *tview.Table
	conexoes       *tview.Table
	eventos        *tview.TextView

	cpuAnterior    cpuAmostra
	cpuTemAnterior bool
	cpuHistorico   []float64

	aoRenovar func()
	pausado   bool
}

func NovoDashboard() *Dashboard {
	d := &Dashboard{
		app:            tview.NewApplication(),
		resumo:         tview.NewTextView().SetDynamicColors(true),
		recursos:       tview.NewTextView().SetDynamicColors(true),
		tabelaAtivos:   tview.NewTable().SetBorders(false),
		tabelaReservas: tview.NewTable().SetBorders(false),
		conexoes:       tview.NewTable().SetBorders(false),
		eventos:        tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetMaxLines(500),
	}
	d.eventos.SetChangedFunc(func() { d.eventos.ScrollToEnd() })

	d.resumo.SetBorder(true).SetTitle(" cyclops-wg — atalhos: q sai · r renova todos os túneis · p pausa/retoma · c limpa eventos ")
	d.recursos.SetBorder(true).SetTitle(" recursos (host) ")
	d.tabelaAtivos.SetBorder(true).SetTitle(" ativos ")
	d.tabelaReservas.SetBorder(true).SetTitle(" reservas ")
	d.conexoes.SetBorder(true).SetTitle(" conexões recentes ")
	d.eventos.SetBorder(true).SetTitle(" eventos ")

	tabelasGateways := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(d.tabelaAtivos, 0, 1, false).
		AddItem(d.tabelaReservas, 0, 1, false)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.resumo, 3, 0, false).
		AddItem(d.recursos, 4, 0, false).
		AddItem(tabelasGateways, 0, 2, false).
		AddItem(d.conexoes, 0, 2, false).
		AddItem(d.eventos, 0, 1, false)

	d.app.SetRoot(layout, true)
	d.app.SetInputCapture(func(evento *tcell.EventKey) *tcell.EventKey {
		switch {
		case evento.Rune() == 'q' || evento.Key() == tcell.KeyCtrlC:
			d.app.Stop()
			return nil
		case evento.Rune() == 'r':
			if d.aoRenovar != nil {
				d.aoRenovar()
			}
			return nil
		case evento.Rune() == 'p':
			d.pausado = !d.pausado
			return nil
		case evento.Rune() == 'c':
			d.eventos.Clear()
			return nil
		}
		return evento
	})

	return d
}

// DefinirAoRenovar registra a função chamada quando o usuário aperta "r".
// Quem chama é responsável por rodar em segundo plano (a rotação demora),
// pra não travar a tela enquanto ela acontece.
func (d *Dashboard) DefinirAoRenovar(f func()) {
	d.aoRenovar = f
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

// Atualiza redesenha o resumo e as tabelas de ativos/reservas a partir de
// uma cópia do estado atual dos slots, tirada com o mutex do controlador
// já liberado.
func (d *Dashboard) Atualiza(inicio time.Time, slots []*Slot) {
	d.app.QueueUpdateDraw(func() {
		var ativos, reservas []*Slot
		ativosOk, reservaOk := 0, 0
		for _, s := range slots {
			if s.Papel == papelAtivo {
				ativos = append(ativos, s)
				if s.Falhas == 0 {
					ativosOk++
				}
			} else {
				reservas = append(reservas, s)
				if s.Falhas == 0 && !s.EmReparo {
					reservaOk++
				}
			}
		}
		sort.Slice(ativos, func(i, j int) bool { return ativos[i].Nome < ativos[j].Nome })
		sort.Slice(reservas, func(i, j int) bool { return reservas[i].Nome < reservas[j].Nome })

		uptime := time.Since(inicio).Round(time.Second)
		pausa := ""
		if d.pausado {
			pausa = "    [yellow]PAUSADO (aperte p pra retomar)[white]"
		}
		d.resumo.SetText(fmt.Sprintf(
			"ativos saudáveis: [green]%d[white]/%d    reservas saudáveis: [green]%d[white]/%d    no ar há %s%s",
			ativosOk, len(ativos), reservaOk, len(reservas), uptime, pausa,
		))

		if d.pausado {
			return
		}
		preencheTabelaGateways(d.tabelaAtivos, ativos)
		preencheTabelaGateways(d.tabelaReservas, reservas)
	})
}

// preencheTabelaGateways redesenha uma das duas tabelas de gateway (ativos
// ou reservas). O papel não precisa de coluna própria: já está no título
// de qual das duas tabelas é essa.
func preencheTabelaGateways(tabela *tview.Table, slots []*Slot) {
	tabela.Clear()
	cabecalho := []string{"GATEWAY", "SERVIDOR", "IP INTERNO", "IP SAÍDA", "ÚLT. TROCA", "FALHAS", "ESTADO"}
	for col, titulo := range cabecalho {
		tabela.SetCell(0, col, tview.NewTableCell(titulo).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false))
	}

	for i, s := range slots {
		estado, cor := "saudável", tcell.ColorGreen
		switch {
		case s.EmReparo:
			estado, cor = "reparando", tcell.ColorYellow
		case s.Falhas > 0:
			estado, cor = "falhando", tcell.ColorRed
		}
		linha := i + 1
		tabela.SetCell(linha, 0, tview.NewTableCell(s.Nome))
		tabela.SetCell(linha, 1, tview.NewTableCell(s.Servidor))
		tabela.SetCell(linha, 2, tview.NewTableCell(s.IP))

		ipSaida := s.IPSaida
		if ipSaida == "" {
			ipSaida = "..."
		}
		tabela.SetCell(linha, 3, tview.NewTableCell(ipSaida).SetTextColor(tcell.ColorAqua))

		troca := "-"
		if !s.UltimaTroca.IsZero() {
			troca = "há " + time.Since(s.UltimaTroca).Round(time.Second).String()
		}
		tabela.SetCell(linha, 4, tview.NewTableCell(troca).SetTextColor(tcell.ColorGray))

		tabela.SetCell(linha, 5, tview.NewTableCell(fmt.Sprintf("%d", s.Falhas)))
		tabela.SetCell(linha, 6, tview.NewTableCell(estado).SetTextColor(cor))
	}
}

// AtualizaConexoes redesenha a tabela de conexões recentes, mais nova primeiro.
// ipSaida mapeia o IP interno de cada gateway (o que aparece em c.Gateway,
// antes dos dois-pontos da porta) pro IP público do túnel WireGuard dele.
func (d *Dashboard) AtualizaConexoes(conexoes []Conexao, ipSaida map[string]string) {
	d.app.QueueUpdateDraw(func() {
		if d.pausado {
			return
		}
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
// recursos, com uma barra ASCII pra cada um e um sparkline do histórico de
// CPU dos últimos quantasAmostrasDeCPU segundos. Usa só caracteres ASCII
// (não os blocos Unicode ▁▂▃.../█/░) porque esses têm largura ambígua e
// bagunçavam o desenho em alguns terminais (ex: dentro de tmux).
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
		if d.pausado {
			return
		}
		linhaCPU := fmt.Sprintf("CPU  %3.0f%%  (%s)   %s", cpuPct, barraBlocos(cpuPct, 20), sparkline(d.cpuHistorico))
		if errCPU != nil {
			linhaCPU = fmt.Sprintf("CPU  --%%  (sem leitura: %v)", errCPU)
		}
		linhaRAM := fmt.Sprintf("RAM  %3.0f%%  (%s)   %.1f/%.1f GB", ramPct, barraBlocos(ramPct, 20), ramUsado, ramTotal)
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
	return strings.Repeat("#", preenchido) + strings.Repeat("-", largura-preenchido)
}

// nivelSparkline é uma rampa ASCII de 8 níveis (baixo pra alto), em vez dos
// blocos Unicode ▁▂▃▄▅▆▇█, que têm largura ambígua e corrompem o desenho
// em terminais/fontes que não os tratam como largura 1.
var nivelSparkline = []rune(".-:=+*#@")

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
