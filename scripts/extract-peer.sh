#!/bin/bash
# Extrai a seção [Peer] dos .conf baixados da Surfshark e grava em outro diretório.
# Só PublicKey, AllowedIPs, Endpoint e PersistentKeepalive passam. A [Interface]
# (PrivateKey, Address, DNS) fica de fora e é montada no runtime a partir do .env.
#
# Uso: scripts/extract-peer.sh <diretorio_origem> <diretorio_destino>

set -u

ORIGEM="${1:-}"
DESTINO="${2:-}"

if [[ -z ${ORIGEM} || -z ${DESTINO} ]]; then
    echo "Uso: $0 {diretorio_origem} {diretorio_destino}"
    exit 1
fi

if [[ ! -d ${ORIGEM} ]]; then
    echo "Diretório de origem não encontrado: ${ORIGEM}"
    exit 1
fi

mkdir -p "${DESTINO}"

convertidos=0
ignorados=0

for arquivo in "${ORIGEM}"/*.conf; do
    [[ -e ${arquivo} ]] || continue
    nome=$(basename "${arquivo}" .conf)
    saida="${DESTINO}/${nome}.conf"

    # Separa no primeiro "=" só: as chaves em base64 terminam em "=" e precisam dele
    peer=$(awk '
        /^\[Peer\]/ { dentro = 1; next }
        /^\[/       { dentro = 0 }
        dentro && /^(PublicKey|AllowedIPs|Endpoint|PersistentKeepalive)/ {
            campo = $1
            sub(/^[^=]*= */, "")
            print campo " = " $0
        }
    ' "${arquivo}")

    # Sem chave pública ou endpoint não há o que guardar
    if ! grep -q '^PublicKey' <<<"${peer}" || ! grep -q '^Endpoint' <<<"${peer}"; then
        echo "Ignorado (sem PublicKey ou Endpoint): ${arquivo}"
        ignorados=$((ignorados + 1))
        continue
    fi

    {
        echo "# Servidor Surfshark ${nome}."
        echo "# Só a seção [Peer]: a [Interface] é montada no runtime a partir do .env."
        echo "[Peer]"
        echo "${peer}"
    } > "${saida}"

    convertidos=$((convertidos + 1))
done

echo "Convertidos: ${convertidos}. Ignorados: ${ignorados}."

# Trava de segurança: nenhuma saída pode ter campo da [Interface]
if grep -Eql '^(PrivateKey|Address|DNS)' "${DESTINO}"/*.conf 2>/dev/null; then
    echo "ERRO: algum arquivo de saída contém PrivateKey, Address ou DNS."
    exit 1
fi
