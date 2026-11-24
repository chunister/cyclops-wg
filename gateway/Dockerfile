FROM debian:13-slim

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
