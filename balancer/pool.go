package main

import "sync"

// Gateway é um upstream SOCKS5: um gateway WireGuard do cyclops-wg.
type Gateway struct {
	Addr string // host:porta do SOCKS5 do gateway

	mu      sync.Mutex
	healthy bool
	fails   int
}

func (g *Gateway) isHealthy() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.healthy
}

// marcaFalha registra uma falha. Depois de atingir o limite, o gateway sai da rotação.
func (g *Gateway) marcaFalha(limite int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fails++
	if g.fails >= limite {
		g.healthy = false
	}
}

// marcaOk zera as falhas e devolve o gateway à rotação.
func (g *Gateway) marcaOk() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fails = 0
	g.healthy = true
}

// Pool escolhe um gateway saudável por conexão, em round-robin.
type Pool struct {
	mu       sync.Mutex
	gateways []*Gateway
	proximo  int
}

func NovoPool(enderecos []string) *Pool {
	p := &Pool{}
	for _, end := range enderecos {
		p.gateways = append(p.gateways, &Gateway{Addr: end, healthy: true})
	}
	return p
}

// Escolhe o próximo gateway saudável. Devolve nil se nenhum estiver de pé.
func (p *Pool) Escolhe() *Gateway {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.gateways)
	if n == 0 {
		return nil
	}
	for i := 0; i < n; i++ {
		g := p.gateways[p.proximo%n]
		p.proximo++
		if g.isHealthy() {
			return g
		}
	}
	return nil
}

func (p *Pool) Todos() []*Gateway {
	p.mu.Lock()
	defer p.mu.Unlock()
	copia := make([]*Gateway, len(p.gateways))
	copy(copia, p.gateways)
	return copia
}

// DefineGateways troca a lista de gateways ativos. Um endereço que já
// existia mantém seu estado de saúde; um endereço novo entra como saudável.
// Usado pelo controlador para promover uma reserva sem reiniciar o balanceador.
func (p *Pool) DefineGateways(enderecos []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	existentes := make(map[string]*Gateway, len(p.gateways))
	for _, g := range p.gateways {
		existentes[g.Addr] = g
	}

	var novaLista []*Gateway
	for _, end := range enderecos {
		if g, ok := existentes[end]; ok {
			novaLista = append(novaLista, g)
		} else {
			novaLista = append(novaLista, &Gateway{Addr: end, healthy: true})
		}
	}
	p.gateways = novaLista
	p.proximo = 0
}
