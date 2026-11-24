#!/bin/bash
# Saudável se houve handshake nos últimos 180 segundos e uma requisição real
# pelo SOCKS5 do próprio gateway funciona. O handshake sozinho não prova que o tráfego passa.

ts=$(wg show wg0 latest-handshakes 2>/dev/null | awk '{ print $2; exit }')
[[ -n ${ts} && ${ts} -gt 0 && $(( $(date +%s) - ts )) -lt 180 ]] || exit 1

curl -s -o /dev/null --max-time 8 \
    --socks5-hostname "127.0.0.1:${SOCKS_PORT:-1080}" \
    "${HEALTH_URL:-https://1.1.1.1}"
