package main

import (
	"os/exec"
	"testing"
	"time"
)

// Sobe o balanceador real (binário compilado em /tmp) e um gateway de
// mentira (também em /tmp, dos testes do balanceador) e confirma que o
// controlador consegue testar a saúde e atualizar a lista de ativos.
func TestIntegracaoComBalanceadorReal(t *testing.T) {
	gw := exec.Command("/tmp/test-gateway")
	gw.Env = append(gw.Environ(), "PORT=1095")
	if err := gw.Start(); err != nil {
		t.Skipf("gateway de teste indisponível: %v", err)
	}
	defer gw.Process.Kill()

	bal := exec.Command("/tmp/cyclops-balancer")
	bal.Env = append(bal.Environ(),
		"GATEWAYS=127.0.0.1:1095",
		"SOCKS_PORT=1096",
		"API_PORT=1097",
		"HEALTH_INTERVAL=30",
	)
	if err := bal.Start(); err != nil {
		t.Skipf("balanceador indisponível: %v", err)
	}
	defer bal.Process.Kill()

	time.Sleep(500 * time.Millisecond)

	if err := testaGateway("127.0.0.1:1095", "1.1.1.1:443", 5*time.Second); err != nil {
		t.Fatalf("testaGateway deveria passar: %v", err)
	}

	if err := AtualizaBalanceador("http://127.0.0.1:1097", []string{"127.0.0.1:1095"}); err != nil {
		t.Fatalf("AtualizaBalanceador falhou: %v", err)
	}

	if err := testaGateway("127.0.0.1:1", "1.1.1.1:443", 300*time.Millisecond); err == nil {
		t.Fatal("testaGateway deveria falhar contra um endereço inexistente")
	}
}
