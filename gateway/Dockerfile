FROM debian:13-slim

# Só pra build: sem isso, o apt-get tenta abrir um frontend interativo do
# debconf (Dialog, Readline, Teletype), não acha TTY nenhum dentro do build
# do Docker, e cospe um aviso de fallback pra cada um até cair sozinho no
# Noninteractive. É só ruído (o build segue normal mesmo sem isso), mas
# evita a confusão e, em teoria, evita travar se algum pacote futuro exigir
# mesmo uma resposta.
ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends wireguard-tools iproute2 microsocks curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY wg-common.sh /usr/local/lib/wg-common.sh
COPY entrypoint.sh /usr/local/bin/entrypoint
COPY switch-server.sh /usr/local/bin/switch-server
COPY healthcheck.sh /usr/local/bin/healthcheck
RUN chmod +x /usr/local/bin/entrypoint /usr/local/bin/switch-server /usr/local/bin/healthcheck

HEALTHCHECK --interval=30s --timeout=10s --start-period=20s --retries=3 CMD ["/usr/local/bin/healthcheck"]
ENTRYPOINT ["/usr/local/bin/entrypoint"]
