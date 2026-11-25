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
- **Controlador** (`controller/`): corre directo en el host, no en un contenedor, y habla con Docker por su API oficial. Levanta `GATEWAY_COUNT` gateways activos más `SPARE_COUNT` de reserva, ya con el túnel abierto. Cuando uno activo falla varias veces seguidas, promueve una reserva sana en su lugar, actualiza el balanceador vía `PUT /gateways`, y cambia el servidor del que falló en segundo plano, respetando un tiempo de espera (`SERVER_COOLDOWN`) antes de reutilizar ese servidor. Con `LATENCY_TOP_SERVERS` activado, esa elección de servidor (en el arranque, en una promoción o en una rotación) queda limitada a los N con menor ping promedio: el controlador pinguea el `Endpoint` de cada servidor del catálogo (sin necesidad de montar el túnel), guarda un promedio móvil en `LATENCY_FILE` y lo actualiza sin parar cada `LATENCY_CHECK_INTERVAL` segundos.

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

El controlador necesita privilegio para hablar con Docker (o que tu usuario esté en el grupo `docker`), y gestiona los contenedores directamente, sin `docker-compose.yml`.

Por defecto, la terminal donde corre el controlador se convierte en un dashboard en vivo: un resumen arriba (cuántos activos y reservas están sanos, cuánto tiempo lleva corriendo), un panel de uso de CPU y RAM del host, más la subida y bajada actuales del proxy (medidas en el propio balanceador, vía `GET /throughput`), dos tablas lado a lado con los gateways activos y los de reserva (servidor, IP interno, IP de salida del WireGuard, ping del último health check con color por rango — verde/amarillo/rojo —, hace cuánto cambió el servidor por última vez, fallos seguidos, estado, que también pasa a "degraded" cuando la conexión está bien pero lenta), una tabla con las últimas conexiones vistas por el balanceador (hora, origen del cliente, destino, qué gateway la atendió y su IP de salida, hasta `CONN_HISTORY` filas), y un panel de eventos abajo con el mismo log que antes iba directo a la terminal. Atajos: `q` o `Ctrl-C` para salir, `r` para disparar al instante una rotación completa (cambia el servidor de todos los gateways a la vez, incluso los que no fallaron); la misma rotación también puede correr sola cada `FULL_ROTATE_INTERVAL` segundos (0 la desactiva, es el valor por defecto); `p` pausa y reanuda la actualización de la pantalla (los contenedores siguen funcionando igual); `c` limpia el panel de eventos; `l` cambia el idioma de la pantalla (inglés, portugués, español), lo mismo que define `DASH_LANG` al arrancar (`en` por defecto). Además de la pantalla, el mismo log también se guarda en `LOG_FILE` (`./controller.log` por defecto, `""` lo desactiva), útil para revisar después de una prueba de carga, cuando el panel de eventos (solo guarda las últimas 500 líneas) o la terminal se vuelven difíciles de seguir en vivo. Para correr sin esta pantalla (por ejemplo detrás de `nohup` o un servicio systemd, sin terminal interactiva), define `DASHBOARD=0` en el `.env` y sigue con `curl -s http://127.0.0.1:9090/status` en su lugar.

## Limitaciones conocidas

El balanceador solo implementa el comando `CONNECT` de SOCKS5, que es TCP. El WebRTC del navegador usa STUN sobre UDP para descubrir la IP pública, y ese tráfico no pasa por el túnel: sale directo a internet, revelando la IP real del usuario aunque el proxy esté configurado.

Probado en [ipleak.net](https://ipleak.net): la IP de salida HTTP y el DNS mostraron el servidor WireGuard correcto, sin fuga. La prueba de WebRTC mostró la IP real del proveedor de internet.

Mitigación disponible hoy: desactiva el WebRTC en el navegador que uses con cyclops-wg. Chromium no tiene un botón nativo para esto; funciona con una extensión que bloquea WebRTC, o cambiando a un navegador que permita desactivarlo por configuración, como Firefox (`about:config`, `media.peerconnection.enabled = false`).

Para una próxima ronda: el balanceador podría ganar soporte para `UDP ASSOCIATE`, o el host podría bloquear el UDP de salida fuera del túnel con una regla de firewall, para tumbar el WebRTC en lugar de dejarlo filtrar.

## Requisitos del host

En Debian 13, Kali GNU/Linux actual o Ubuntu, `sudo scripts/install-deps.sh` instala todo esto de una vez (Docker, Compose v2, Go y el módulo WireGuard). En otra distro, o si prefieres instalar a mano, aquí está la lista:

- Kernel Linux con el módulo WireGuard cargado (`sudo modprobe wireguard`). El propio `wireguard-tools` (`wg`) solo se usa dentro del contenedor del gateway, instalado por su propio `Dockerfile` — no hace falta instalarlo en el host, pero la [página de instalación](https://www.wireguard.com/install/) está ahí si quieres probarlo fuera del proyecto. Probado en Kali GNU/Linux 2025.4, kernel `6.16.8+kali-amd64`.
- [Docker Engine](https://docs.docker.com/engine/install/) 27 o más reciente, con Compose v2 (para la prueba rápida) y acceso al socket de Docker (para el controlador). Probado con Docker `27.5.1+dfsg4` y Docker Compose `v2.32.4-3`.
- [Go](https://go.dev/doc/install), solo para compilar el controlador. El `go.mod` pide Go 1.26 (traído por el SDK de Docker); con `GOTOOLCHAIN=auto` (el valor por defecto desde Go 1.21), el propio `go build` descarga el toolchain correcto si el instalado es más viejo. Probado con `go1.26.3`.
- Cuenta en [Surfshark](https://surfshark.com/), para generar la clave WireGuard usada en el `.env` (ver "Configuración" más abajo).

## Licencia

Por definir. Si se reutiliza código del proyecto original, hay que mantener el aviso de copyright que exige su licencia BSD-2-Clause.
