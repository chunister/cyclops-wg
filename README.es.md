# cyclops-wg

[Português](README.md) · [English](README.en.md)

Gateways WireGuard con entrada SOCKS5 y rotación de salida en tiempo de ejecución.

Un único punto de entrada SOCKS5 reparte las conexiones entre varios contenedores. Cada contenedor mantiene un túnel WireGuard hacia un servidor distinto. Puedes cambiar los servidores sin reconstruir la imagen y sin detener el stack.

## Origen

La idea surgió del proyecto [tatu-rotate](https://github.com/marciosouza20/tatu-rotate), que usa OpenVPN de Surfshark con Privoxy y Mubeng. Este proyecto cambia la arquitectura: usa WireGuard, SOCKS5 nativo y control en tiempo de ejecución.

## Estado

Probado de punta a punta en un Kali: túnel WireGuard real, el balanceador SOCKS5 funcionando con curl y con Burp, cambio de servidor sin detener el contenedor, y el controlador levantando el conjunto completo (10 gateways activos más 3 de reserva) contra un Docker real. La promoción automática también se probó matando un gateway activo a propósito: una reserva ocupó su lugar, y el gateway caído se reinició y se apuntó a otro servidor en segundo plano.

## Piezas

- **Gateway** (`gateway/`): un contenedor por salida, con WireGuard, microsocks y una comprobación de la IP de salida mediante `healthcheck.sh`. El `switch-server.sh`, ejecutado con `docker exec`, cambia el servidor del túnel sin reiniciar el contenedor.
- **Balanceador** (`balancer/`): servidor SOCKS5 propio en Go (sin autenticación, solo el comando CONNECT). Acepta conexiones de cualquier cliente SOCKS5, como Burp o curl, y elige un gateway sano por conexión, en round-robin. Prueba cada gateway cada `HEALTH_INTERVAL` segundos y saca de la rotación al que falle `HEALTH_FAILS` veces seguidas. Expone `GET /status` con el estado de cada gateway, y `PUT /gateways` para cambiar la lista de activos sin reiniciar.
- **Controlador** (`controller/`): corre directo en el host, no en un contenedor, y habla con Docker por su API oficial. Levanta `GATEWAY_COUNT` gateways activos más `SPARE_COUNT` de reserva, ya con el túnel abierto. Cuando uno activo falla varias veces seguidas, promueve una reserva sana en su lugar, actualiza el balanceador vía `PUT /gateways`, y cambia el servidor del que falló en segundo plano, respetando un tiempo de espera (`SERVER_COOLDOWN`) antes de reutilizar ese servidor.

## Stack

- Lenguaje: Go, tanto en el balanceador como en el controlador.
- Gateway: Debian 13 slim con `wireguard-tools` y `microsocks`.
- Orquestación: la API de Docker en Go (`github.com/docker/docker/client`), no `docker compose`, porque el controlador necesita crear e inspeccionar contenedores de forma dinámica.

## Configuración

1. Copia `.env.example` a `.env`.
2. Genera la clave WireGuard en el panel de Surfshark y completa `WG_PRIVATE_KEY` y `WG_ADDRESS` (el `Address` del `.conf` descargado).
3. Descarga los `.conf` de los servidores que quieras usar y corre `scripts/extract-peer.sh` para generar los archivos de `servers/` (los archivos que ya están en esa carpeta muestran el formato esperado).

### Prueba rápida con 1 gateway

Completa también `GATEWAY1_SERVER` en el `.env`, con el nombre de un archivo de `servers/` sin la extensión. Luego:

```bash
docker compose up --build -d
curl -s --socks5-hostname 127.0.0.1:1080 http://ip-api.com/json
```

### Ejecución completa, con reserva automática

Necesita `GATEWAY_COUNT + SPARE_COUNT` servidores distintos en `servers/` (10 + 3 por defecto). Carga el módulo WireGuard en el host (`sudo modprobe wireguard`), exporta las variables del `.env` y corre el controlador:

```bash
cd controller
go build -o controller .
set -a && source ../.env && set +a
sudo -E ./controller
```

El controlador necesita privilegio para hablar con Docker (o que tu usuario esté en el grupo `docker`), y gestiona los contenedores directamente, sin `docker-compose.yml`. Sigue `curl -s http://127.0.0.1:9090/status` para ver el papel (activo o reserva) y la salud de cada gateway.

## Limitaciones conocidas

El balanceador solo implementa el comando `CONNECT` de SOCKS5, que es TCP. El WebRTC del navegador usa STUN sobre UDP para descubrir la IP pública, y ese tráfico no pasa por el túnel: sale directo a internet, revelando la IP real del usuario aunque el proxy esté configurado.

Probado en [ipleak.net](https://ipleak.net): la IP de salida HTTP y el DNS mostraron el servidor WireGuard correcto, sin fuga. La prueba de WebRTC mostró la IP real del proveedor de internet.

Mitigación disponible hoy: desactiva el WebRTC en el navegador que uses con cyclops-wg. Chromium no tiene un botón nativo para esto; funciona con una extensión que bloquea WebRTC, o cambiando a un navegador que permita desactivarlo por configuración, como Firefox (`about:config`, `media.peerconnection.enabled = false`).

Para una próxima ronda: el balanceador podría ganar soporte para `UDP ASSOCIATE`, o el host podría bloquear el UDP de salida fuera del túnel con una regla de firewall, para tumbar el WebRTC en lugar de dejarlo filtrar.

## Requisitos del host

- Kernel con el módulo WireGuard cargado (`sudo modprobe wireguard`).
- Docker Engine 27 o más reciente, con Compose v2 (para la prueba rápida) y acceso al socket de Docker (para el controlador). Probado con Docker 27.5.1 en Kali.
- Go, solo para compilar el controlador. El `go.mod` pide Go 1.26 (traído por el SDK de Docker); con `GOTOOLCHAIN=auto` (el valor por defecto desde Go 1.21), el propio `go build` descarga el toolchain correcto si el instalado es más viejo.

## Licencia

Por definir. Si se reutiliza código del proyecto original, hay que mantener el aviso de copyright que exige su licencia BSD-2-Clause.
