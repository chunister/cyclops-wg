#!/bin/bash
# Instala as dependências do host pra rodar o cyclops-wg: Docker, Go e o
# módulo WireGuard do kernel. Testado em Debian 13 (netinst), Kali GNU/Linux
# atual e Ubuntu. Qualquer outra distro baseada em apt deve funcionar, mas
# só essas três foram verificadas de verdade.
#
# Uso: sudo scripts/install-deps.sh
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
    echo "precisa rodar como root (sudo scripts/install-deps.sh)" >&2
    exit 1
fi

if ! command -v apt-get >/dev/null 2>&1; then
    echo "não achei o apt-get: este script só cobre distros baseadas em Debian (Debian, Kali, Ubuntu)." >&2
    exit 1
fi

distro="desconhecida"
if [[ -r /etc/os-release ]]; then
    # shellcheck source=/dev/null
    source /etc/os-release
    distro="${PRETTY_NAME:-${NAME:-desconhecida}}"
fi
echo "distro detectada: ${distro}"

echo "atualizando a lista de pacotes..."
apt-get update

# Num Debian netinst mínimo, estes três podem nem existir ainda.
echo "instalando dependências básicas..."
apt-get install -y --no-install-recommends curl ca-certificates gnupg

echo "instalando Docker e Go..."
apt-get install -y --no-install-recommends docker.io golang-go

# O pacote do Compose v2 muda de nome conforme a distro: Kali e Ubuntu usam
# "docker-compose-v2", o Debian trixie usa só "docker-compose" (mesmo nome
# do v1 antigo, mas já é o binário em Go da v2).
echo "instalando o Compose v2..."
if apt-cache show docker-compose-v2 >/dev/null 2>&1; then
    apt-get install -y --no-install-recommends docker-compose-v2
else
    apt-get install -y --no-install-recommends docker-compose
fi

echo "carregando o módulo WireGuard..."
modprobe wireguard

echo "deixando o módulo WireGuard carregado também depois de reiniciar..."
if ! grep -qx "wireguard" /etc/modules-load.d/wireguard.conf 2>/dev/null; then
    echo "wireguard" > /etc/modules-load.d/wireguard.conf
fi

echo "garantindo que o serviço do Docker está ativo..."
systemctl enable --now docker

echo
echo "pronto. versões instaladas:"
docker --version
docker compose version
go version
uname -r

echo
echo "próximo passo: copie .env.example para .env e preencha WG_PRIVATE_KEY e WG_ADDRESS (veja o README)."
echo "se o seu usuário não for root, rode 'usermod -aG docker \$(whoami)' e reabra a sessão, ou use sudo nos comandos do projeto."
