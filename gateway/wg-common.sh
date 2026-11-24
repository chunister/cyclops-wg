#!/bin/bash
# Funções compartilhadas pelo entrypoint e pelo switch-server.
# Em vez do wg-quick, monta o túnel com ip e wg. Assim não precisa de iptables nem de resolvconf.

ESTADO=/run/wg-estado
SERVIDORES="${SERVIDORES:-/servers}"

# Lê um campo do arquivo [Peer]: peer_valor <campo> <arquivo>
peer_valor() {
    # Tira só o primeiro "=": as chaves em base64 terminam em "=" e precisam dele
    awk -v k="$1" '$1 == k { sub(/^[^=]*= */, ""); print; exit }' "$2"
}

# Cria a interface wg0 com a chave e o endereço do .env. A rota padrão fica para depois.
criar_interface() {
    mkdir -p "${ESTADO}"
    (umask 077 && printf '%s' "${WG_PRIVATE_KEY}" > "${ESTADO}/chave")
    ip link add wg0 type wireguard
    wg set wg0 private-key "${ESTADO}/chave"
    ip -4 address add "${WG_ADDRESS}" dev wg0
    ip link set wg0 up
}

# Aponta o túnel para um servidor de /servers. Chamar de novo troca o servidor sem derrubar o container.
aplicar_servidor() {
    local nome="$1"
    local conf="${SERVIDORES}/${nome}.conf"

    if [[ ! -f ${conf} ]]; then
        echo "Servidor não encontrado: ${nome}" >&2
        return 1
    fi

    local pub endpoint permitidos host porta ip_destino
    pub=$(peer_valor PublicKey "${conf}")
    endpoint=$(peer_valor Endpoint "${conf}")
    permitidos=$(peer_valor AllowedIPs "${conf}")
    host=${endpoint%:*}
    porta=${endpoint##*:}

    ip_destino=$(getent ahostsv4 "${host}" | awk 'NR == 1 { print $1 }')
    if [[ -z ${ip_destino} ]]; then
        echo "Não consegui resolver ${host}" >&2
        return 1
    fi

    # Guarda a rota original antes de a rota padrão passar para o wg0
    if [[ ! -f ${ESTADO}/saida ]]; then
        ip -4 route show default | awk '{ print $3 " " $5; exit }' > "${ESTADO}/saida"
    fi
    local gw dev
    read -r gw dev < "${ESTADO}/saida"

    # O tráfego para o servidor VPN precisa continuar saindo pela rede do Docker
    ip -4 route replace "${ip_destino}" via "${gw}" dev "${dev}"
    wg set wg0 peer "${pub}" endpoint "${ip_destino}:${porta}" allowed-ips "${permitidos}" persistent-keepalive 25

    # Remove o servidor anterior só depois de o novo estar configurado
    if [[ -f ${ESTADO}/pub && $(cat "${ESTADO}/pub") != "${pub}" ]]; then
        wg set wg0 peer "$(cat "${ESTADO}/pub")" remove
        ip -4 route del "$(cat "${ESTADO}/ip")" 2>/dev/null || true
    fi

    echo "${pub}" > "${ESTADO}/pub"
    echo "${ip_destino}" > "${ESTADO}/ip"
    echo "${nome}" > "${ESTADO}/servidor"
}

# Gera tráfego e espera o handshake com o servidor atual
esperar_handshake() {
    local i ts agora
    for i in $(seq 1 15); do
        curl -s -o /dev/null --max-time 2 https://1.1.1.1 || true
        ts=$(wg show wg0 latest-handshakes | awk '{ print $2; exit }')
        agora=$(date +%s)
        if [[ -n ${ts} && ${ts} -gt 0 && $((agora - ts)) -lt 30 ]]; then
            return 0
        fi
    done
    return 1
}
