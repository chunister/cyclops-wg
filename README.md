# 👁️ cyclops-wg

[English](README.en.md) · [Español](README.es.md)

Gateways WireGuard com entrada SOCKS5 e troca de saída em tempo de execução.

Um ponto de entrada SOCKS5 distribui as conexões entre vários containers. Cada container mantém um túnel WireGuard para um servidor diferente. Os servidores podem ser trocados sem rebuild da imagem e sem derrubar o stack.

## 🌱 Origem

A ideia veio do projeto [tatu-rotate](https://github.com/marciosouza20/tatu-rotate), que usa OpenVPN da Surfshark com Privoxy e Mubeng. Este projeto muda a arquitetura: usa WireGuard, SOCKS5 nativo e controle em tempo de execução.

## 📋 Requisitos no host

Em Debian 13, Kali GNU/Linux atual ou Ubuntu, `sudo scripts/install-deps.sh` instala tudo isto de uma vez (Docker, Compose v2, Go e o módulo WireGuard). Em outra distro, ou se preferir instalar na mão, segue a lista:

- Kernel Linux com o módulo WireGuard carregado (`sudo modprobe wireguard`). O `wireguard-tools` (`wg`) em si só é usado dentro do container do gateway, instalado pelo próprio `Dockerfile`. Não precisa instalar no host, mas a [página de instalação](https://www.wireguard.com/install/) existe se quiser testar fora do projeto. Testado em Kali GNU/Linux 2025.4, kernel `6.16.8+kali-amd64`.
- [Docker Engine](https://docs.docker.com/engine/install/) 27 ou mais recente, com Compose v2 (para o teste rápido) e acesso ao socket do Docker (para o controlador). Testado com Docker `27.5.1+dfsg4` e Docker Compose `v2.32.4-3`.
- [Go](https://go.dev/doc/install), só para compilar o controlador. O `go.mod` pede Go 1.26 (puxado pelo SDK do Docker); com `GOTOOLCHAIN=auto` (padrão do Go 1.21+), o próprio `go build` baixa o toolchain certo se o instalado for mais antigo. Testado com `go1.26.3`.
- Conta na [Surfshark](https://surfshark.com/), pra gerar a chave WireGuard usada no `.env` (veja "Configuração" abaixo). Pra criar a conta com um bônus de indicação, use [este link](https://surfshark.club/friend/adjVkuXB) ("You got a gift from a friend").

## ✅ Status

Testado de ponta a ponta no Kali e em um Debian 13: túnel WireGuard real, balanceador SOCKS5 funcionando com curl e com o Burp, troca de servidor sem derrubar o container, e o controlador subindo o conjunto completo (10 gateways ativos mais 3 reserva) contra um Docker de verdade. A promoção automática também foi testada derrubando um gateway ativo de propósito:

```bash
docker kill cyclops-gw-1
```

Uma reserva assumiu o lugar dele, e o gateway derrubado foi reiniciado e apontado para outro servidor em segundo plano.

## 🧩 Peças

- **Gateway** (`gateway/`): um container por saída, com WireGuard, microsocks e uma verificação do IP de saída via `healthcheck.sh`. O `switch-server.sh`, rodado com `docker exec`, troca o servidor do túnel sem reiniciar o container.
- **Balanceador** (`balancer/`): servidor SOCKS5 próprio em Go (sem autenticação, só o comando CONNECT). Recebe conexões de qualquer cliente SOCKS5, como o Burp ou o curl, e escolhe um gateway saudável por conexão, em round-robin. Testa cada gateway a cada `HEALTH_INTERVAL` segundos e tira da rotação o que falhar `HEALTH_FAILS` vezes seguidas. Expõe `GET /status` com o estado de cada gateway, e `PUT /gateways` para trocar a lista de ativos sem reiniciar.
- **Controlador** (`controller/`): roda direto no host, não em container, e fala com o Docker pela API oficial. Sobe `GATEWAY_COUNT` gateways ativos mais `SPARE_COUNT` reservas, já com túnel aberto. Quando um ativo falha repetidas vezes, promove uma reserva saudável no lugar dele, atualiza o balanceador via `PUT /gateways`, e troca o servidor do que falhou em segundo plano, respeitando um tempo de espera (`SERVER_COOLDOWN`) antes de reaproveitar aquele servidor. Com `LATENCY_TOP_SERVERS` ligado, essa escolha de servidor (no boot, numa promoção ou numa rotação) fica restrita aos N com menor ping médio: o controlador pinga o `Endpoint` de cada servidor do catálogo (sem precisar montar o túnel), guarda uma média móvel em `LATENCY_FILE` e atualiza ela sem parar a cada `LATENCY_CHECK_INTERVAL` segundos.

## 🛠️ Stack

- Linguagem: Go, tanto no balanceador quanto no controlador.
- Gateway: Debian 13 slim com `wireguard-tools` e `microsocks`.
- Orquestração: a API do Docker em Go (`github.com/docker/docker/client`), não `docker compose`, porque o controlador precisa criar e inspecionar containers de forma dinâmica.

## ⚙️ Configuração

1. Copie `.env.example` para `.env`.
2. Gere a chave WireGuard no painel da Surfshark e preencha `WG_PRIVATE_KEY` no `.env`. O `WG_ADDRESS` já vem com o valor que a Surfshark costuma atribuir (`10.14.0.2/16`); confira se bate com o `Address` do seu `.conf` baixado e só troque se for diferente.
3. Os servidores em `servers/` já vêm prontos no repositório. Só baixe `.conf` novos e rode `scripts/extract-peer.sh` se quiser adicionar servidores além dos que já estão lá.

### ⚡ Teste rápido com 1 gateway

Preencha também `GATEWAY1_SERVER` no `.env`, com o nome de um arquivo de `servers/` sem a extensão. Depois:

```bash
docker compose up --build -d
curl -s --socks5-hostname 127.0.0.1:1080 http://ip-api.com/json
```

### 🚀 Execução completa, com reserva automática

Exige `GATEWAY_COUNT + SPARE_COUNT` servidores diferentes em `servers/` (10 + 3 por padrão). Carregue o módulo WireGuard no host (`sudo modprobe wireguard`), exporte as variáveis do `.env` e rode o controlador:

```bash
cd controller
go build -o controller .
set -a && source ../.env && set +a
sudo -E ./controller
```

O controlador precisa de privilégio para falar com o Docker (ou que seu usuário esteja no grupo `docker`), e vai gerenciar os containers direto, sem `docker-compose.yml`.

Por padrão, o terminal onde o controlador roda vira um dashboard ao vivo: um resumo no topo (quantos ativos e reservas estão saudáveis, há quanto tempo está no ar), um painel de uso de CPU e RAM do host, mais upload e download atuais do proxy (medidos no próprio balanceador, via `GET /throughput`), duas tabelas lado a lado com os gateways ativos e os reserva (servidor, IP interno, IP de saída do WireGuard, ping do último health check com cor por faixa (verde, amarelo, vermelho), há quanto tempo o servidor mudou pela última vez, falhas seguidas, estado, que também vira "degraded" quando a conexão está ok mas lenta), uma tabela com as últimas conexões vistas pelo balanceador (hora, origem do cliente, destino, qual gateway atendeu e o IP de saída dele, até `CONN_HISTORY` linhas), e um painel de eventos embaixo com o mesmo log que antes ia pro terminal cru. Atalhos: `q` ou `Ctrl-C` abre uma confirmação com três opções: só sair (os containers continuam rodando e o proxy continua no ar), sair e remover todos os containers, ou cancelar; `r` pede confirmação (padrão "não") antes de disparar uma rotação completa (troca o servidor de todos os gateways de uma vez, mesmo os que não falharam); a mesma rotação também pode rodar sozinha a cada `FULL_ROTATE_INTERVAL` segundos (0 desliga, é o padrão); `p` pausa e retoma a atualização da tela (os containers continuam normalmente); `c` limpa o painel de eventos; `l`/`L` troca o idioma da tela pra frente/trás (inglês, português, espanhol), o mesmo que `DASH_LANG` define no arranque (`en` por padrão, pensando em publicar o projeto em inglês); `t`/`T` troca a paleta de cores do dashboard pra frente/trás, entre 10 temas (o original, Dracula, Nord, Solarized Dark, Gruvbox Dark, alto contraste, Monokai, One Dark, Tokyo Night e Catppuccin Mocha). O idioma e o tema atuais aparecem no canto direito da primeira linha do resumo, e ficam salvos em `PREFS_FILE` (`./dashboard-prefs.json` por padrão) pra abrir no mesmo estado na próxima execução; depois da primeira troca pelo teclado, esse arquivo passa a valer mais que `DASH_LANG`. Além da tela, o mesmo log também é gravado em `LOG_FILE` (`./controller.log` por padrão, `""` desliga), útil pra revisar depois de um teste de carga, quando o painel de eventos (só guarda as últimas 500 linhas) ou o terminal ficam difíceis de acompanhar ao vivo. Para rodar sem essa tela (por exemplo, atrás de `nohup` ou um serviço systemd, sem terminal interativo), defina `DASHBOARD=0` no `.env` e acompanhe pelo `curl -s http://127.0.0.1:9090/status` em vez disso.

## ⚠️ Limitações conhecidas

O balanceador só implementa o comando `CONNECT` do SOCKS5, que é TCP. O WebRTC do navegador usa STUN sobre UDP para descobrir o IP público, e esse tráfego não passa pelo túnel: ele sai direto pela internet, revelando o IP real do usuário mesmo com o proxy configurado.

Testado em [ipleak.net](https://ipleak.net): o IP de saída HTTP e o DNS mostraram o servidor WireGuard correto, sem vazamento. O teste de WebRTC mostrou o IP real do provedor de internet.

Mitigação disponível hoje: desative o WebRTC no navegador que for usar com o cyclops-wg. No Chromium não existe um botão nativo para isso; funciona com uma extensão que bloqueia WebRTC, ou trocando para um navegador que permita desativar por configuração, como o Firefox (`about:config`, `media.peerconnection.enabled = false`).

Para uma próxima rodada: o balanceador pode ganhar suporte a `UDP ASSOCIATE`, ou o host pode bloquear UDP de saída fora do túnel por regra de firewall, para derrubar o WebRTC em vez de deixá-lo vazar.

## 📄 Licença

[GPLv3](LICENSE). Se código do projeto original for reaproveitado, é preciso manter o aviso de copyright exigido pela licença BSD-2-Clause do original.
