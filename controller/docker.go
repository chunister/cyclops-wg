package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
)

// RemoveSeExistir tira um container antigo do caminho, se houver, para que o
// controlador possa recriar o ambiente do zero numa nova execução.
func (d *Docker) RemoveSeExistir(ctx context.Context, nome string) error {
	err := d.cli.ContainerRemove(ctx, nome, container.RemoveOptions{Force: true})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

type Docker struct {
	cli *client.Client
	cfg Config
}

func NovoDocker(cfg Config) (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Docker{cli: cli, cfg: cfg}, nil
}

// GaranteRede cria a rede do projeto se ela não existir. É nela que o
// controlador resolve o IP de cada gateway, e os gateways resolvem o balanceador.
func (d *Docker) GaranteRede(ctx context.Context) error {
	redes, err := d.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}
	for _, r := range redes {
		if r.Name == d.cfg.NetworkName {
			return nil
		}
	}
	_, err = d.cli.NetworkCreate(ctx, d.cfg.NetworkName, network.CreateOptions{Driver: "bridge"})
	return err
}

// ConstroiImagens roda o build das duas imagens via o CLI do docker, mais
// simples do que falar com a API de build diretamente.
func ConstroiImagens(cfg Config, raiz string) error {
	builds := []struct{ dir, tag string }{
		{filepath.Join(raiz, "gateway"), cfg.GatewayImage},
		{filepath.Join(raiz, "balancer"), cfg.BalancerImage},
	}
	for _, b := range builds {
		cmd := exec.Command("docker", "build", "-t", b.tag, b.dir)
		cmd.Stdout = logWriter{}
		cmd.Stderr = logWriter{}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("build de %s falhou: %w", b.tag, err)
		}
	}
	return nil
}

// CriaGateway sobe um container de gateway apontando para um servidor.
func (d *Docker) CriaGateway(ctx context.Context, nome, servidor string) error {
	cfg := &container.Config{
		Image: d.cfg.GatewayImage,
		Env: []string{
			"WG_PRIVATE_KEY=" + d.cfg.WGPrivateKey,
			"WG_ADDRESS=" + d.cfg.WGAddress,
			"SERVER=" + servidor,
			"SOCKS_PORT=1090",
		},
	}
	hostCfg := &container.HostConfig{
		CapAdd: []string{"NET_ADMIN"},
		Binds:  []string{absPath(d.cfg.ServersDir) + ":/servers:ro"},
		DNS:    []string{"162.252.172.57", "149.154.159.92"},
	}
	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			d.cfg.NetworkName: {},
		},
	}
	resp, err := d.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, nome)
	if err != nil {
		return err
	}
	return d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
}

// CriaBalanceador sobe o balanceador na rede do projeto, publicando SOCKS_PORT e API_PORT no host.
func (d *Docker) CriaBalanceador(ctx context.Context) error {
	cfg := &container.Config{
		Image: d.cfg.BalancerImage,
		Env: []string{
			"SOCKS_PORT=1080",
			"API_PORT=8081",
			fmt.Sprintf("HEALTH_INTERVAL=%d", d.cfg.HealthInterval),
			fmt.Sprintf("HEALTH_FAILS=%d", d.cfg.HealthFails),
		},
		ExposedPorts: nat.PortSet{
			"1080/tcp": struct{}{},
			"8081/tcp": struct{}{},
		},
	}
	hostCfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			"1080/tcp": {{HostIP: "0.0.0.0", HostPort: d.cfg.SocksPort}},
			"8081/tcp": {{HostIP: "0.0.0.0", HostPort: d.cfg.ApiPort}},
		},
	}
	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			d.cfg.NetworkName: {},
		},
	}
	resp, err := d.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, "cyclops-balancer")
	if err != nil {
		return err
	}
	return d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
}

// IPNaRede devolve o IP do container na rede do projeto, para o controlador
// falar com o microsocks dele (porta 1090) sem precisar publicar porta nenhuma.
func (d *Docker) IPNaRede(ctx context.Context, nome string) (string, error) {
	info, err := d.cli.ContainerInspect(ctx, nome)
	if err != nil {
		return "", err
	}
	rede, ok := info.NetworkSettings.Networks[d.cfg.NetworkName]
	if !ok || rede.IPAddress == "" {
		return "", fmt.Errorf("container %s sem IP na rede %s", nome, d.cfg.NetworkName)
	}
	return rede.IPAddress, nil
}

// TrocaServidor executa o switch-server dentro do container, sem reiniciá-lo.
func (d *Docker) TrocaServidor(ctx context.Context, nome, servidor string, timeout time.Duration) error {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	criado, err := d.cli.ContainerExecCreate(execCtx, nome, container.ExecOptions{
		Cmd:          []string{"switch-server", servidor},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return err
	}
	resp, err := d.cli.ContainerExecAttach(execCtx, criado.ID, container.ExecStartOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()

	// Drena a saída para o exec terminar; não precisamos do conteúdo aqui.
	buf := make([]byte, 4096)
	for {
		if _, err := resp.Reader.Read(buf); err != nil {
			break
		}
	}

	inspecao, err := d.cli.ContainerExecInspect(execCtx, criado.ID)
	if err != nil {
		return err
	}
	if inspecao.ExitCode != 0 {
		return fmt.Errorf("switch-server saiu com código %d", inspecao.ExitCode)
	}
	return nil
}

// ReiniciaGateway é o último recurso quando a troca de servidor não resolve.
func (d *Docker) ReiniciaGateway(ctx context.Context, nome string) error {
	timeout := 10
	return d.cli.ContainerRestart(ctx, nome, container.StopOptions{Timeout: &timeout})
}

type logWriter struct{}

func (logWriter) Write(p []byte) (int, error) {
	logPrintf("%s", p)
	return len(p), nil
}
