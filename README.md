# cyclops-wg

[English](README.en.md) · [Español](README.es.md)

Gateways WireGuard com entrada SOCKS5 e troca de saída em tempo de execução.

Um ponto de entrada SOCKS5 distribui as conexões entre vários containers. Cada container mantém um túnel WireGuard para um servidor diferente. Os servidores podem ser trocados sem rebuild da imagem e sem derrubar o stack.

## Origem

A ideia veio do projeto [tatu-rotate](https://github.com/marciosouza20/tatu-rotate), que usa OpenVPN da Surfshark com Privoxy e Mubeng. Este projeto muda a arquitetura: usa WireGuard, SOCKS5 nativo e controle em tempo de execução.

## Status

Testado de ponta a ponta num Kali: túnel WireGuard real, balanceador SOCKS5 funcionando com curl e com o Burp, e troca de servidor sem derrubar o container. O controlador que sobe o conjunto completo (gateways ativos e reserva) ainda não foi testado com Docker de verdade.

## Peças

- **Gateway** (`gateway/`): um container por saída, com WireGuard, microsocks e uma verificação do IP de saída via `healthcheck.sh`. O `switch-server.sh`, rodado com `docker exec`, troca o servidor do túnel sem reiniciar o container.
- **Balanceador** (`balancer/`): servidor SOCKS5 próprio em Go (sem autenticação, só o comando CONNECT). Recebe conexões de qualquer cliente SOCKS5, como o Burp ou o curl, e escolhe um gateway saudável por conexão, em round-robin. Testa cada gateway a cada `HEALTH_INTERVAL` segundos e tira da rotação o que falhar `HEALTH_FAILS` vezes seguidas. Expõe `GET /status` com o estado de cada gateway, e `PUT /gateways` para trocar a lista de ativos sem reiniciar.
- **Controlador** (`controller/`): roda direto no host, não em container, e fala com o Docker pela API oficial. Sobe `GATEWAY_COUNT` gateways ativos mais `SPARE_COUNT` reservas, já com túnel aberto. Quando um ativo falha repetidas vezes, promove uma reserva saudável no lugar dele, atualiza o balanceador via `PUT /gateways`, e troca o servidor do que falhou em segundo plano, respeitando um tempo de espera (`SERVER_COOLDOWN`) antes de reaproveitar aquele servidor.

## Stack

- Linguagem: Go, tanto no balanceador quanto no controlador.
- Gateway: Debian 13 slim com `wireguard-tools` e `microsocks`.
- Orquestração: a API do Docker em Go (`github.com/docker/docker/client`), não `docker compose`, porque o controlador precisa criar e inspecionar containers de forma dinâmica.

## Configuração

1. Copie `.env.example` para `.env`.
2. Gere a chave WireGuard no painel da Surfshark e preencha `WG_PRIVATE_KEY` e `WG_ADDRESS` (o `Address` do `.conf` baixado).
3. Baixe os `.conf` dos servidores que quiser usar e rode `scripts/extract-peer.sh` para gerar os arquivos de `servers/` (veja os arquivos já incluídos nessa pasta como exemplo de formato).

### Teste rápido com 1 gateway

Preencha também `GATEWAY1_SERVER` no `.env`, com o nome de um arquivo de `servers/` sem a extensão. Depois:

```bash
docker compose up --build -d
curl -s --socks5-hostname 127.0.0.1:1080 http://ip-api.com/json
```

### Execução completa, com reserva automática

Exige `GATEWAY_COUNT + SPARE_COUNT` servidores diferentes em `servers/` (10 + 3 por padrão). Carregue o módulo WireGuard no host (`sudo modprobe wireguard`), exporte as variáveis do `.env` e rode o controlador:

```bash
cd controller
go build -o controller .
set -a && source ../.env && set +a
sudo -E ./controller
```

O controlador precisa de privilégio para falar com o Docker (ou que seu usuário esteja no grupo `docker`), e vai gerenciar os containers direto, sem `docker-compose.yml`. Acompanhe `curl -s http://127.0.0.1:9090/status` para ver o papel (ativo ou reserva) e a saúde de cada gateway.

## Limitações conhecidas

O balanceador só implementa o comando `CONNECT` do SOCKS5, que é TCP. O WebRTC do navegador usa STUN sobre UDP para descobrir o IP público, e esse tráfego não passa pelo túnel: ele sai direto pela internet, revelando o IP real do usuário mesmo com o proxy configurado.

Testado em [ipleak.net](https://ipleak.net): o IP de saída HTTP e o DNS mostraram o servidor WireGuard correto, sem vazamento. O teste de WebRTC mostrou o IP real do provedor de internet.

Mitigação disponível hoje: desative o WebRTC no navegador que for usar com o cyclops-wg. No Chromium não existe um botão nativo para isso; funciona com uma extensão que bloqueia WebRTC, ou trocando para um navegador que permita desativar por configuração, como o Firefox (`about:config`, `media.peerconnection.enabled = false`).

Para uma próxima rodada: o balanceador pode ganhar suporte a `UDP ASSOCIATE`, ou o host pode bloquear UDP de saída fora do túnel por regra de firewall, para derrubar o WebRTC em vez de deixá-lo vazar. O controlador também ainda não foi testado com containers de verdade, só com testes de unidade e integração contra o balanceador real.

## Requisitos no host

- Kernel com o módulo WireGuard carregado (`sudo modprobe wireguard`).
- Docker com Compose v2 (para o teste rápido) e acesso ao socket do Docker (para o controlador).
- Go, só para compilar o controlador. O `go.mod` pede uma versão recente (puxada pelo SDK do Docker); com `GOTOOLCHAIN=auto` (padrão do Go 1.21+), o próprio `go build` baixa o toolchain certo se o instalado for mais antigo.

## Licença

A definir. Se código do projeto original for reaproveitado, é preciso manter o aviso de copyright exigido pela licença BSD-2-Clause do original.
