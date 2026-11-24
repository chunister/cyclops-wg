#!/bin/bash
# Sobe o túnel WireGuard e o servidor SOCKS5 do gateway.
set -euo pipefail

# shellcheck source=wg-common.sh
source /usr/local/lib/wg-common.sh

: "${WG_PRIVATE_KEY:?Defina WG_PRIVATE_KEY no .env}"
: "${SERVER:?Defina SERVER com o nome de um arquivo de /servers}"
WG_ADDRESS="${WG_ADDRESS:-10.14.0.2/16}"
SOCKS_PORT="${SOCKS_PORT:-1080}"

criar_interface
aplicar_servidor "${SERVER}"

# Com o servidor configurado, todo o tráfego passa a sair pelo túnel.
# Se o túnel cair, o tráfego é descartado em vez de sair pela rede do Docker.
ip -4 route replace default dev wg0

if ! esperar_handshake; then
    echo "Sem handshake com ${SERVER}" >&2
    exit 1
fi

echo "Túnel ativo: ${SERVER}"
exec microsocks -i 0.0.0.0 -p "${SOCKS_PORT}"
