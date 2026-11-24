# cyclops-wg

[English](README.en.md) · [Español](README.es.md)

Gateways WireGuard com entrada SOCKS5 e troca de saída em tempo de execução.

Um ponto de entrada SOCKS5 distribui as conexões entre vários containers. Cada container mantém um túnel WireGuard para um servidor diferente. Os servidores podem ser trocados sem rebuild da imagem e sem derrubar o stack.

## Origem

A ideia veio do projeto [tatu-rotate](https://github.com/marciosouza20/tatu-rotate), que usa OpenVPN da Surfshark com Privoxy e Mubeng. Este projeto muda a arquitetura: usa WireGuard, SOCKS5 nativo e controle em tempo de execução.

## Status

Em construção. Ainda não há código funcional.

## Plano

- Gateway: um container por saída, com WireGuard, microsocks e uma verificação do IP de saída.
- Balanceador SOCKS5: recebe as conexões na porta de entrada e escolhe um gateway saudável para cada uma.
- API de controle: troca o servidor de um gateway sem rebuild, drenando as conexões antes da troca.
- Configuração: o `.env.example` é versionado. O `.env` e os arquivos `.conf` do WireGuard ficam fora do git.

## Stack

- Linguagem: Go. O balanceador compila em um binário estático.
- Gateway: Debian 13 slim com `wireguard-tools` e `microsocks`.
- Balanceador: servidor SOCKS5 próprio (sem autenticação, só o comando CONNECT). Recebe conexões de qualquer cliente SOCKS5, como o Burp ou o curl, e escolhe um gateway saudável por conexão, em round-robin. Testa cada gateway a cada `HEALTH_INTERVAL` segundos e tira da rotação o que falhar `HEALTH_FAILS` vezes seguidas. Expõe `GET /status` com o estado de cada gateway.

## Configuração

1. Copie `.env.example` para `.env`.
2. Gere a chave WireGuard no painel da Surfshark e preencha `WG_PRIVATE_KEY`.
3. Liste os servidores desejados em `SERVERS`.

A documentação completa de geração de chaves e de arquivos `.conf` virá quando o gateway estiver pronto.

## Limitações conhecidas

O balanceador só implementa o comando `CONNECT` do SOCKS5, que é TCP. O WebRTC do navegador usa STUN sobre UDP para descobrir o IP público, e esse tráfego não passa pelo túnel: ele sai direto pela internet, revelando o IP real do usuário mesmo com o proxy configurado.

Testado em [ipleak.net](https://ipleak.net): o IP de saída HTTP e o DNS mostraram o servidor WireGuard correto, sem vazamento. O teste de WebRTC mostrou o IP real do provedor de internet.

Mitigação disponível hoje: desative o WebRTC no navegador que for usar com o cyclops-wg. No Chromium não existe um botão nativo para isso; funciona com uma extensão que bloqueia WebRTC, ou trocando para um navegador que permita desativar por configuração, como o Firefox (`about:config`, `media.peerconnection.enabled = false`).

Para uma próxima rodada: o balanceador pode ganhar suporte a `UDP ASSOCIATE`, ou o host pode bloquear UDP de saída fora do túnel por regra de firewall, para derrubar o WebRTC em vez de deixá-lo vazar.

## Requisitos no host

- Kernel com o módulo WireGuard carregado (`sudo modprobe wireguard`).
- Docker com Compose v2.

## Licença

A definir. Se código do projeto original for reaproveitado, é preciso manter o aviso de copyright exigido pela licença BSD-2-Clause do original.
