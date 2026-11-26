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

// Dashboard mostra o estado do controlador no terminal, ao vivo, no lugar
// do log cru. É montado só quando DASHBOARD está ligado.
type Dashboard struct {
	app             *tview.Application
	pages           *tview.Pages
	resumo          *tview.TextView
	recursos        *tview.TextView
	tabelasGateways *tview.Flex
	tabelaAtivos    *tview.Table
	tabelaReservas  *tview.Table
	conexoes        *tview.Table
	eventos         *tview.TextView
	atalhos         *tview.TextView

	cpuAnterior    cpuAmostra
	cpuTemAnterior bool

	linhaCPU string
	linhaRAM string
	linhaNet string

	bytesUpAnterior, bytesDownAnterior int64
	temAmostraDeRede                   bool
	amostraDeRedeQuando                time.Time

	temaIndice int

	aoRenovar func()
	aoSair    func(removerContainers bool)
	pausado   bool
}

// tema devolve a paleta ativa no momento.
func (d *Dashboard) tema() Tema {
	return temas[d.temaIndice]
}

func NovoDashboard() *Dashboard {
	d := &Dashboard{
		app:            tview.NewApplication(),
		resumo:         tview.NewTextView().SetDynamicColors(true),
		recursos:       tview.NewTextView().SetDynamicColors(true),
		tabelaAtivos:   tview.NewTable().SetBorders(false).SetFixed(1, 0),
		tabelaReservas: tview.NewTable().SetBorders(false).SetFixed(1, 0),
		conexoes:       tview.NewTable().SetBorders(false).SetFixed(1, 0),
		eventos:        tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetMaxLines(500),
		atalhos:        tview.NewTextView().SetDynamicColors(true),
	}
	d.eventos.SetChangedFunc(func() { d.eventos.ScrollToEnd() })

	d.resumo.SetBorder(true).SetTitle(" cyclops-wg ")
	d.recursos.SetBorder(true)
	d.tabelaAtivos.SetBorder(true)
	d.tabelaReservas.SetBorder(true)
	d.conexoes.SetBorder(true)
	d.eventos.SetBorder(true)
	d.atalhos.SetBorder(true)

	d.tabelasGateways = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(d.tabelaAtivos, 0, 1, false).
		AddItem(d.tabelaReservas, 0, 1, false)
	d.tabelasGateways.SetBorder(true)

	d.aplicaIdioma()
	d.aplicaTema()

	rodape := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(d.eventos, 0, 3, false).
		AddItem(d.atalhos, 0, 1, false)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.resumo, 3, 0, false).
		AddItem(d.recursos, 5, 0, false).
		AddItem(d.tabelasGateways, 0, 2, false).
		AddItem(d.conexoes, 0, 2, false).
		AddItem(rodape, 0, 1, false)

	d.pages = tview.NewPages().AddPage("main", layout, true, true)
	d.app.SetRoot(d.pages, true)
	d.app.SetInputCapture(func(evento *tcell.EventKey) *tcell.EventKey {
		if d.pages.HasPage("sair") {
			// Com o modal de confirmação aberto, deixa só ele tratar o
			// teclado (setas/enter pra escolher o botão); nenhum atalho do
			// dashboard roda por baixo enquanto a pergunta está na tela.
			return evento
		}
		switch {
		case evento.Rune() == 'q' || evento.Key() == tcell.KeyCtrlC:
			d.mostraConfirmacaoDeSaida()
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
		case evento.Rune() == 'l':
			ProximoIdioma()
			d.aplicaIdioma()
			return nil
		case evento.Rune() == 'L':
			AnteriorIdioma()
			d.aplicaIdioma()
			return nil
		case evento.Rune() == 't':
			d.temaIndice = (d.temaIndice + 1) % len(temas)
			d.aplicaTema()
			return nil
		case evento.Rune() == 'T':
			d.temaIndice = (d.temaIndice - 1 + len(temas)) % len(temas)
			d.aplicaTema()
			return nil
		}
		return evento
	})

	return d
}

// aplicaIdioma redesenha os títulos e textos estáticos dos painéis no
// idioma ativo. Chamada na montagem do dashboard e de novo toda vez que o
// atalho "l" troca o idioma, já que esses textos não são redesenhados
// pelos tickers normais (ao contrário dos cabeçalhos de tabela, que são
// recalculados em toda atualização).
func (d *Dashboard) aplicaIdioma() {
	d.recursos.SetTitle(T("panel_resources"))
	d.tabelasGateways.SetTitle(T("panel_gateways"))
	d.tabelaAtivos.SetTitle(T("panel_active"))
	d.tabelaReservas.SetTitle(T("panel_spare"))
	d.conexoes.SetTitle(T("panel_connections"))
	d.eventos.SetTitle(T("panel_events"))
	d.atalhos.SetTitle(T("panel_shortcuts"))
	d.redesenhaAtalhos()
}

// redesenhaAtalhos monta o texto do painel de atalhos: a tecla na cor de
// cabeçalho do tema, a descrição na cor de texto normal. Separado da
// tradução (shortcut_*) porque a cor depende do tema, não do idioma — e os
// dois podem mudar em momentos diferentes.
func (d *Dashboard) redesenhaAtalhos() {
	t := d.tema()
	corChave := fmt.Sprintf("#%06x", t.Cabecalho.Hex())
	corDesc := fmt.Sprintf("#%06x", t.Texto.Hex())
	linha := func(tecla, chave string) string {
		return fmt.Sprintf("[%s]%s[%s]  %s", corChave, tecla, corDesc, T(chave))
	}
	d.atalhos.SetText(strings.Join([]string{
		linha("q", "shortcut_quit"),
		linha("r", "shortcut_renew"),
		linha("p", "shortcut_pause"),
		linha("c", "shortcut_clear"),
		linha("l/L", "shortcut_language"),
		linha("t/T", "shortcut_theme"),
	}, "\n"))
}

// caixaTematizavel é o conjunto de setters de cor que tview promove de Box
// pra qualquer widget que o embute (TextView, Table, Flex), usado aqui só
// pra poder repetir a aplicação de cor num slice em vez de uma linha por
// painel.
type caixaTematizavel interface {
	SetBackgroundColor(tcell.Color) *tview.Box
	SetBorderColor(tcell.Color) *tview.Box
	SetTitleColor(tcell.Color) *tview.Box
}

// aplicaTema redesenha fundo, borda e título de cada painel na paleta
// ativa, e atualiza os defaults globais do tview (Styles) usados por
// qualquer célula de tabela nova (preencheTabelaGateways/AtualizaConexoes
// recriam todas as células a cada atualização, então pegam a cor nova no
// próximo ciclo automaticamente). Chamada na montagem do dashboard e de
// novo toda vez que o atalho "t"/"T" troca o tema.
func (d *Dashboard) aplicaTema() {
	t := d.tema()

	tview.Styles.PrimitiveBackgroundColor = t.Fundo
	tview.Styles.ContrastBackgroundColor = t.Fundo
	tview.Styles.PrimaryTextColor = t.Texto
	tview.Styles.BorderColor = t.Apagado
	tview.Styles.TitleColor = t.Branco
	tview.Styles.GraphicsColor = t.Apagado

	d.pages.SetBackgroundColor(t.Fundo)
	for _, caixa := range []caixaTematizavel{
		d.resumo, d.recursos, d.tabelasGateways, d.tabelaAtivos,
		d.tabelaReservas, d.conexoes, d.eventos, d.atalhos,
	} {
		caixa.SetBackgroundColor(t.Fundo).SetBorderColor(t.Apagado).SetTitleColor(t.Branco)
	}
	d.redesenhaAtalhos()
}

// DefinirAoRenovar registra a função chamada quando o usuário aperta "r".
// Quem chama é responsável por rodar em segundo plano (a rotação demora),
// pra não travar a tela enquanto ela acontece.
func (d *Dashboard) DefinirAoRenovar(f func()) {
	d.aoRenovar = f
}

// DefinirAoSair registra a função chamada quando o usuário confirma a saída
// pelo modal de "q". removerContainers indica se o usuário escolheu também
// derrubar os containers (gateways + balanceador) ou só fechar a tela e
// deixar tudo rodando. Quem chama roda isso de forma síncrona, antes de
// parar o app: remover os containers pode levar alguns segundos.
func (d *Dashboard) DefinirAoSair(f func(removerContainers bool)) {
	d.aoSair = f
}

// mostraConfirmacaoDeSaida substitui a tela principal por um modal com três
// opções, em vez de sair na hora: evita derrubar o proxy sem querer com um
// "q" apertado por engano, já que os containers continuam valendo mesmo com
// o dashboard fechado.
func (d *Dashboard) mostraConfirmacaoDeSaida() {
	t := d.tema()
	modal := tview.NewModal().
		SetText(T("confirm_quit_text")).
		SetBackgroundColor(t.Fundo).
		SetTextColor(t.Texto).
		SetButtonBackgroundColor(t.Apagado).
		SetButtonTextColor(t.Branco).
		AddButtons([]string{T("confirm_quit_only"), T("confirm_quit_remove"), T("confirm_quit_cancel")}).
		SetDoneFunc(func(indice int, rotulo string) {
			switch indice {
			case 0: // só sair
				if d.aoSair != nil {
					d.aoSair(false)
				}
				d.app.Stop()
			case 1: // sair e remover os containers
				if d.aoSair != nil {
					d.aoSair(true)
				}
				d.app.Stop()
			default: // cancelar
				d.pages.RemovePage("sair")
			}
		})
	d.pages.AddPage("sair", modal, true, true)
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
			pausa = T("paused_suffix")
		}
		linhaResumo := T("summary_line", ativosOk, len(ativos), reservaOk, len(reservas), uptime, pausa)
		d.resumo.SetText(linhaResumo + d.rotuloTemaIdioma(linhaResumo))

		if d.pausado {
			return
		}
		tema := d.tema()
		preencheTabelaGateways(d.tabelaAtivos, ativos, tema)
		preencheTabelaGateways(d.tabelaReservas, reservas, tema)
	})
}

// rotuloTemaIdioma monta "tema · idioma" pra mostrar no canto direito da
// primeira linha do resumo, preenchido com espaços a partir da largura real
// do painel (GetInnerRect) pra ficar encostado na borda direita. Antes do
// primeiro desenho a largura ainda pode vir zerada; nesse caso só um espaço
// separa o texto, e o alinhamento se acerta sozinho no próximo ciclo.
func (d *Dashboard) rotuloTemaIdioma(linhaEsquerda string) string {
	t := d.tema()
	rotulo := fmt.Sprintf("[#%06x]%s · %s[-]", t.Apagado.Hex(), t.Nome, nomeIdioma(IdiomaAtual()))
	tamanhoVisivel := len(t.Nome) + 3 + len(nomeIdioma(IdiomaAtual()))

	_, _, largura, _ := d.resumo.GetInnerRect()
	preenchimento := largura - len(linhaEsquerda) - tamanhoVisivel
	if preenchimento < 1 {
		preenchimento = 1
	}
	return strings.Repeat(" ", preenchimento) + rotulo
}

// preencheTabelaGateways redesenha uma das duas tabelas de gateway (ativos
// ou reservas). O papel não precisa de coluna própria: já está no título
// de qual das duas tabelas é essa.
func preencheTabelaGateways(tabela *tview.Table, slots []*Slot, tema Tema) {
	tabela.Clear()
	cabecalho := []string{
		T("col_gateway"), T("col_server"), T("col_internal_ip"), T("col_exit_ip"),
		T("col_ping"), T("col_last_switch"), T("col_failures"), T("col_state"),
	}
	for col, titulo := range cabecalho {
		tabela.SetCell(0, col, tview.NewTableCell(titulo).
			SetTextColor(tema.Cabecalho).
			SetSelectable(false))
	}

	for i, s := range slots {
		estado, cor := T("state_healthy"), tema.Saudavel
		switch {
		case s.EmReparo:
			estado, cor = T("state_repairing"), tema.Degradado
		case s.Falhas > 0:
			estado, cor = T("state_failing"), tema.Falhando
		case s.PingMs >= limiarPingRuimMs:
			// Conexão ok (sem falhas), mas lenta o bastante pra avisar sem
			// tirar o gateway da rotação — isso quem decide é o HEALTH_FAILS.
			estado, cor = T("state_degraded"), tema.Degradado
		}
		linha := i + 1
		tabela.SetCell(linha, 0, tview.NewTableCell(s.Nome).SetTextColor(tema.Texto))
		tabela.SetCell(linha, 1, tview.NewTableCell(s.Servidor).SetTextColor(tema.Texto))
		tabela.SetCell(linha, 2, tview.NewTableCell(s.IP).SetTextColor(tema.Texto))

		ipSaida := s.IPSaida
		if ipSaida == "" {
			ipSaida = "..."
		}
		tabela.SetCell(linha, 3, tview.NewTableCell(ipSaida).SetTextColor(tema.Destaque))

		ping := "-"
		if s.PingMs > 0 {
			ping = fmt.Sprintf("%dms", s.PingMs)
		}
		tabela.SetCell(linha, 4, tview.NewTableCell(ping).SetTextColor(corPing(s.PingMs, tema)))

		troca := "-"
		if !s.UltimaTroca.IsZero() {
			troca = T("time_ago", duracaoResumida(time.Since(s.UltimaTroca)))
		}
		tabela.SetCell(linha, 5, tview.NewTableCell(troca).SetTextColor(tema.Apagado))

		tabela.SetCell(linha, 6, tview.NewTableCell(fmt.Sprintf("%d", s.Falhas)).SetTextColor(tema.Texto))
		tabela.SetCell(linha, 7, tview.NewTableCell(estado).SetTextColor(cor))
	}
}

// limiarPingBomMs e limiarPingRuimMs marcam as faixas de cor da coluna de
// ping: abaixo do primeiro fica verde, entre os dois fica amarelo, acima do
// segundo fica vermelho. São só um ponto de partida razoável pra túneis
// WireGuard de distâncias bem variadas; ajuste se achar que não bate com o
// que você considera "bom" ou "ruim" na sua rede.
const (
	limiarPingBomMs  = 150
	limiarPingRuimMs = 350
)

// corPing escolhe a cor da coluna de ping pelo mesmo princípio da coluna de
// estado: um jeito de bater o olho e já saber se tá bom, no limite ou ruim,
// sem precisar comparar números.
func corPing(ms int64, tema Tema) tcell.Color {
	switch {
	case ms <= 0:
		return tema.Apagado
	case ms < limiarPingBomMs:
		return tema.Saudavel
	case ms < limiarPingRuimMs:
		return tema.Degradado
	default:
		return tema.Falhando
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
		tema := d.tema()
		d.conexoes.Clear()
		cabecalho := []string{T("col_time"), T("col_source"), T("col_destination"), T("col_gateway"), T("col_exit_ip")}
		for col, titulo := range cabecalho {
			d.conexoes.SetCell(0, col, tview.NewTableCell(titulo).
				SetTextColor(tema.Cabecalho).
				SetSelectable(false))
		}
		for i, c := range conexoes {
			linha := i + 1
			d.conexoes.SetCell(linha, 0, tview.NewTableCell(c.Quando.Format("15:04:05")).
				SetTextColor(tema.Apagado))
			d.conexoes.SetCell(linha, 1, tview.NewTableCell(c.Origem).
				SetTextColor(tema.Branco))
			d.conexoes.SetCell(linha, 2, tview.NewTableCell(c.Destino).
				SetTextColor(tema.Alerta))
			d.conexoes.SetCell(linha, 3, tview.NewTableCell(c.Gateway).
				SetTextColor(tema.Destaque))

			saida := "..."
			if host, _, err := net.SplitHostPort(c.Gateway); err == nil {
				if ip := ipSaida[host]; ip != "" {
					saida = ip
				}
			}
			d.conexoes.SetCell(linha, 4, tview.NewTableCell(saida).
				SetTextColor(tema.Destaque))
		}
	})
}

// AtualizaRecursos lê o uso de CPU e RAM do host e redesenha o painel de
// recursos, com uma barra ASCII pra cada um. Usa só caracteres ASCII (não os
// blocos Unicode ▁▂▃.../█/░) porque esses têm largura ambígua e bagunçavam o
// desenho em alguns terminais (ex: dentro de tmux).
func (d *Dashboard) AtualizaRecursos() {
	atual, errCPU := lerCPUAmostra()
	var cpuPct float64
	if errCPU == nil {
		if d.cpuTemAnterior {
			cpuPct = percentualCPU(d.cpuAnterior, atual)
		}
		d.cpuAnterior = atual
		d.cpuTemAnterior = true
	}
	ramPct, ramUsado, ramTotal, errRAM := usoRAM()

	d.app.QueueUpdateDraw(func() {
		if d.pausado {
			return
		}
		d.linhaCPU = fmt.Sprintf("CPU  %3.0f%%  (%s)", cpuPct, barraBlocos(cpuPct, 20))
		if errCPU != nil {
			d.linhaCPU = fmt.Sprintf("CPU  --%%  %s", T("no_reading", errCPU))
		}
		d.linhaRAM = fmt.Sprintf("RAM  %3.0f%%  (%s)   %.1f/%.1f GB", ramPct, barraBlocos(ramPct, 20), ramUsado, ramTotal)
		if errRAM != nil {
			d.linhaRAM = fmt.Sprintf("RAM  --%%  %s", T("no_reading", errRAM))
		}
		d.redesenhaRecursos()
	})
}

// AtualizaThroughput recebe os totais acumulados de bytes do balanceador e
// calcula a taxa de upload/download comparando com a leitura anterior, do
// mesmo jeito que AtualizaRecursos calcula o uso de CPU a partir de duas
// amostras de /proc/stat.
func (d *Dashboard) AtualizaThroughput(uploadBytes, downloadBytes int64) {
	agora := time.Now()
	d.app.QueueUpdateDraw(func() {
		if d.temAmostraDeRede {
			decorrido := agora.Sub(d.amostraDeRedeQuando).Seconds()
			if decorrido > 0 {
				up := float64(uploadBytes-d.bytesUpAnterior) / decorrido
				down := float64(downloadBytes-d.bytesDownAnterior) / decorrido
				if up < 0 {
					up = 0
				}
				if down < 0 {
					down = 0
				}
				d.linhaNet = fmt.Sprintf("NET  up %s/s  down %s/s", formataTaxa(up), formataTaxa(down))
			}
		}
		d.bytesUpAnterior = uploadBytes
		d.bytesDownAnterior = downloadBytes
		d.amostraDeRedeQuando = agora
		d.temAmostraDeRede = true

		if !d.pausado {
			d.redesenhaRecursos()
		}
	})
}

// redesenhaRecursos junta as linhas de CPU, RAM e rede já formatadas num
// único SetText. Só pode ser chamada de dentro de um QueueUpdateDraw que já
// esteja rodando: chamar QueueUpdateDraw de novo aqui de dentro travaria,
// porque o canal só é lido pelo próprio loop que estaria bloqueado chamando
// esta função.
func (d *Dashboard) redesenhaRecursos() {
	texto := d.linhaCPU + "\n" + d.linhaRAM
	if d.linhaNet != "" {
		texto += "\n" + d.linhaNet
	}
	d.recursos.SetText(texto)
}

// formataTaxa escolhe a unidade (B, KB, MB por segundo) de acordo com a
// grandeza, em vez de sempre mostrar um número de bytes cru.
func formataTaxa(bytesPorSegundo float64) string {
	switch {
	case bytesPorSegundo >= 1<<20:
		return fmt.Sprintf("%.1f MB", bytesPorSegundo/(1<<20))
	case bytesPorSegundo >= 1<<10:
		return fmt.Sprintf("%.1f KB", bytesPorSegundo/(1<<10))
	default:
		return fmt.Sprintf("%.0f B", bytesPorSegundo)
	}
}

// duracaoResumida formata uma duração sem o nível de detalhe de segundos
// quando já passou mais de um minuto. time.Duration.String() sempre inclui
// segundos (ex: "5m0s"), o que fica poluído numa coluna que atualiza a
// cada segundo.
func duracaoResumida(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
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

// Executa bloqueia rodando o loop de interface até o usuário sair (q ou ctrl-c).
func (d *Dashboard) Executa() error {
	return d.app.Run()
}
