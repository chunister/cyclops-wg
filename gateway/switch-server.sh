#!/bin/bash
# Troca o servidor do túnel sem reiniciar o container.
# Uso: docker exec <gateway> switch-server <nome>
set -euo pipefail

# shellcheck source=wg-common.sh
source /usr/local/lib/wg-common.sh

novo="${1:?Uso: switch-server <nome>}"

aplicar_servidor "${novo}"

if ! esperar_handshake; then
    echo "Sem handshake com ${novo}" >&2
    exit 1
fi

echo "Servidor atual: ${novo}"
