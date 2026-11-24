# cyclops-wg

Gateways WireGuard com entrada SOCKS5 e troca de saída em tempo de execução.

Um único ponto de entrada SOCKS5 distribui conexões entre vários containers, cada um com um túnel WireGuard para um servidor diferente. Os servidores podem ser trocados sem rebuild da imagem e sem derrubar o stack.

## Origem

A ideia partiu do projeto [tatu-rotate](https://github.com/marciosouza20/tatu-rotate), que usa OpenVPN da Surfshark com Privoxy e Mubeng. Este projeto reescreve a arquitetura com WireGuard, SOCKS5 nativo e controle em tempo de execução.

## Status

Em construção. Ainda não há código funcional.

## Stack

- **Linguagem:** Go (balanceador SOCKS5 e API de controle, em um binário estático).
- **Gateway:** Debian slim com `wireguard-tools` e `microsocks`.

## Plano

- **Gateway:** um container por saída, com WireGuard, microsocks e um healthcheck do IP de saída.
- **Balanceador SOCKS5:** escuta na porta de entrada e escolhe um gateway saudável por conexão.
- **API de controle:** troca o servidor de um gateway sem rebuild, com drenagem antes da troca.
- **Configuração:** `.env.example` versionado; `.env` e os `.conf` WireGuard ficam fora do git.

## Configuração

1. Copie `.env.example` para `.env`.
2. Gere a chave WireGuard no painel da Surfshark e preencha `WG_PRIVATE_KEY`.
3. Liste os servidores desejados em `SERVERS`.

Os detalhes de geração das chaves e dos `.conf` serão documentados quando o gateway estiver pronto.

## Requisitos no host

- Kernel com o módulo WireGuard carregado (`sudo modprobe wireguard`).
- Docker com Compose v2.

## Licença

A definir. Se código do projeto original for reaproveitado, manter o aviso de copyright exigido pela licença BSD-2-Clause do original.
