package main

import "github.com/gdamore/tcell/v2"

// Tema define a cor de cada papel semântico usado no dashboard: fundo, texto
// normal, texto em destaque (títulos de painel), cabeçalho de coluna, os
// três níveis de saúde (mesma lógica das colunas PING e STATE), o tom do IP
// de saída, o do destino de uma conexão, e um tom apagado pra bordas e
// timestamps.
type Tema struct {
	Nome      string
	Fundo     tcell.Color
	Texto     tcell.Color
	Branco    tcell.Color
	Cabecalho tcell.Color
	Saudavel  tcell.Color
	Degradado tcell.Color
	Falhando  tcell.Color
	Destaque  tcell.Color
	Alerta    tcell.Color
	Apagado   tcell.Color
}

// temas é a lista de paletas disponíveis, cicladas pelo atalho "t"/"T" do
// dashboard. A primeira (índice 0) é o visual original, sem nenhuma cor
// nova: trocar de tema nunca é obrigatório.
var temas = []Tema{
	{
		Nome: "Default", Fundo: tcell.ColorBlack, Texto: tcell.ColorSilver, Branco: tcell.ColorWhite,
		Cabecalho: tcell.ColorYellow, Saudavel: tcell.ColorGreen, Degradado: tcell.ColorYellow,
		Falhando: tcell.ColorRed, Destaque: tcell.ColorAqua, Alerta: tcell.ColorOrange, Apagado: tcell.ColorGray,
	},
	{
		Nome: "Dracula", Fundo: tcell.GetColor("#282A36"), Texto: tcell.GetColor("#F8F8F2"), Branco: tcell.GetColor("#F8F8F2"),
		Cabecalho: tcell.GetColor("#F1FA8C"), Saudavel: tcell.GetColor("#50FA7B"), Degradado: tcell.GetColor("#F1FA8C"),
		Falhando: tcell.GetColor("#FF5555"), Destaque: tcell.GetColor("#BD93F9"), Alerta: tcell.GetColor("#FFB86C"), Apagado: tcell.GetColor("#6272A4"),
	},
	{
		Nome: "Nord", Fundo: tcell.GetColor("#2E3440"), Texto: tcell.GetColor("#D8DEE9"), Branco: tcell.GetColor("#ECEFF4"),
		Cabecalho: tcell.GetColor("#EBCB8B"), Saudavel: tcell.GetColor("#A3BE8C"), Degradado: tcell.GetColor("#EBCB8B"),
		Falhando: tcell.GetColor("#BF616A"), Destaque: tcell.GetColor("#88C0D0"), Alerta: tcell.GetColor("#D08770"), Apagado: tcell.GetColor("#4C566A"),
	},
	{
		Nome: "Solarized Dark", Fundo: tcell.GetColor("#002B36"), Texto: tcell.GetColor("#839496"), Branco: tcell.GetColor("#EEE8D5"),
		Cabecalho: tcell.GetColor("#B58900"), Saudavel: tcell.GetColor("#859900"), Degradado: tcell.GetColor("#B58900"),
		Falhando: tcell.GetColor("#DC322F"), Destaque: tcell.GetColor("#268BD2"), Alerta: tcell.GetColor("#CB4B16"), Apagado: tcell.GetColor("#586E75"),
	},
	{
		Nome: "Gruvbox Dark", Fundo: tcell.GetColor("#282828"), Texto: tcell.GetColor("#EBDBB2"), Branco: tcell.GetColor("#FBF1C7"),
		Cabecalho: tcell.GetColor("#FABD2F"), Saudavel: tcell.GetColor("#B8BB26"), Degradado: tcell.GetColor("#FABD2F"),
		Falhando: tcell.GetColor("#FB4934"), Destaque: tcell.GetColor("#83A598"), Alerta: tcell.GetColor("#FE8019"), Apagado: tcell.GetColor("#928374"),
	},
	{
		Nome: "Alto Contraste", Fundo: tcell.ColorBlack, Texto: tcell.ColorWhite, Branco: tcell.ColorWhite,
		Cabecalho: tcell.ColorYellow, Saudavel: tcell.ColorLime, Degradado: tcell.ColorYellow,
		Falhando: tcell.ColorRed, Destaque: tcell.ColorAqua, Alerta: tcell.ColorOrange, Apagado: tcell.ColorSilver,
	},
	{
		Nome: "Monokai", Fundo: tcell.GetColor("#272822"), Texto: tcell.GetColor("#F8F8F2"), Branco: tcell.GetColor("#F9F8F5"),
		Cabecalho: tcell.GetColor("#E6DB74"), Saudavel: tcell.GetColor("#A6E22E"), Degradado: tcell.GetColor("#E6DB74"),
		Falhando: tcell.GetColor("#F92672"), Destaque: tcell.GetColor("#66D9EF"), Alerta: tcell.GetColor("#FD971F"), Apagado: tcell.GetColor("#75715E"),
	},
	{
		Nome: "One Dark", Fundo: tcell.GetColor("#282C34"), Texto: tcell.GetColor("#ABB2BF"), Branco: tcell.GetColor("#DCDFE4"),
		Cabecalho: tcell.GetColor("#E5C07B"), Saudavel: tcell.GetColor("#98C379"), Degradado: tcell.GetColor("#E5C07B"),
		Falhando: tcell.GetColor("#E06C75"), Destaque: tcell.GetColor("#61AFEF"), Alerta: tcell.GetColor("#D19A66"), Apagado: tcell.GetColor("#5C6370"),
	},
	{
		Nome: "Tokyo Night", Fundo: tcell.GetColor("#1A1B26"), Texto: tcell.GetColor("#A9B1D6"), Branco: tcell.GetColor("#C0CAF5"),
		Cabecalho: tcell.GetColor("#E0AF68"), Saudavel: tcell.GetColor("#9ECE6A"), Degradado: tcell.GetColor("#E0AF68"),
		Falhando: tcell.GetColor("#F7768E"), Destaque: tcell.GetColor("#7AA2F7"), Alerta: tcell.GetColor("#FF9E64"), Apagado: tcell.GetColor("#565F89"),
	},
	{
		Nome: "Catppuccin Mocha", Fundo: tcell.GetColor("#1E1E2E"), Texto: tcell.GetColor("#CDD6F4"), Branco: tcell.GetColor("#F5E0DC"),
		Cabecalho: tcell.GetColor("#F9E2AF"), Saudavel: tcell.GetColor("#A6E3A1"), Degradado: tcell.GetColor("#F9E2AF"),
		Falhando: tcell.GetColor("#F38BA8"), Destaque: tcell.GetColor("#89B4FA"), Alerta: tcell.GetColor("#FAB387"), Apagado: tcell.GetColor("#6C7086"),
	},
}
