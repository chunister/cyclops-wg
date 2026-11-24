# cyclops-wg

[Português](README.md) · [Español](README.es.md)

WireGuard gateways with a SOCKS5 entry point and runtime exit rotation.

A single SOCKS5 entry point spreads connections across several containers. Each container keeps a WireGuard tunnel to a different server. You can change servers without rebuilding the image and without stopping the stack.

## Origin

The idea comes from [tatu-rotate](https://github.com/marciosouza20/tatu-rotate), which uses Surfshark OpenVPN with Privoxy and Mubeng. This project changes the architecture: it uses WireGuard, native SOCKS5, and runtime control.

## Status

Tested end to end on a Kali box: a real WireGuard tunnel, the SOCKS5 balancer working with curl and with Burp, and server switching without stopping the container. The controller that brings up the full set (active and spare gateways) hasn't been tested against a real Docker daemon yet.

## Pieces

- **Gateway** (`gateway/`): one container per exit, with WireGuard, microsocks, and an exit-IP check via `healthcheck.sh`. `switch-server.sh`, run with `docker exec`, changes the tunnel's server without restarting the container.
- **Balancer** (`balancer/`): its own SOCKS5 server in Go (no authentication, CONNECT only). It accepts connections from any SOCKS5 client, such as Burp or curl, and picks a healthy gateway per connection, round-robin. It tests each gateway every `HEALTH_INTERVAL` seconds and drops one from rotation after `HEALTH_FAILS` failures in a row. It exposes `GET /status` with each gateway's state, and `PUT /gateways` to swap the active list without restarting.
- **Controller** (`controller/`): runs directly on the host, not in a container, and talks to Docker through its official API. It brings up `GATEWAY_COUNT` active gateways plus `SPARE_COUNT` spares, already tunneled. When an active gateway keeps failing, it promotes a healthy spare in its place, updates the balancer via `PUT /gateways`, and switches the failed one's server in the background, respecting a cooldown (`SERVER_COOLDOWN`) before that server is reused.

## Stack

- Language: Go, for both the balancer and the controller.
- Gateway: Debian 13 slim with `wireguard-tools` and `microsocks`.
- Orchestration: the Docker API in Go (`github.com/docker/docker/client`), not `docker compose`, because the controller needs to create and inspect containers dynamically.

## Setup

1. Copy `.env.example` to `.env`.
2. Generate a WireGuard key in the Surfshark dashboard and fill in `WG_PRIVATE_KEY` and `WG_ADDRESS` (the downloaded `.conf`'s `Address`).
3. Download the `.conf` files for the servers you want and run `scripts/extract-peer.sh` to generate the files in `servers/` (the files already in that folder show the expected format).

### Quick test with 1 gateway

Also fill in `GATEWAY1_SERVER` in `.env`, with the name of a file in `servers/` without the extension. Then:

```bash
docker compose up --build -d
curl -s --socks5-hostname 127.0.0.1:1080 http://ip-api.com/json
```

### Full run, with automatic spares

Needs `GATEWAY_COUNT + SPARE_COUNT` distinct servers in `servers/` (10 + 3 by default). Load the WireGuard module on the host (`sudo modprobe wireguard`), export the variables from `.env`, and run the controller:

```bash
cd controller
go build -o controller .
source ../.env && export $(cut -d= -f1 ../.env)
export SERVERS_DIR=../servers
sudo -E ./controller
```

The controller needs enough privilege to talk to Docker (or your user in the `docker` group), and manages containers directly, without `docker-compose.yml`. Watch `curl -s http://127.0.0.1:9090/status` to see each gateway's role (active or spare) and health.

## Known limitations

The balancer only implements the SOCKS5 `CONNECT` command, which is TCP. The browser's WebRTC uses STUN over UDP to discover the public IP, and that traffic doesn't go through the tunnel: it leaves directly over the internet, revealing the real user IP even with the proxy configured.

Tested on [ipleak.net](https://ipleak.net): the HTTP exit IP and DNS both showed the correct WireGuard server, with no leak. The WebRTC test showed the real ISP IP.

Mitigation available today: disable WebRTC in the browser you use with cyclops-wg. Chromium has no native toggle for this; it works with an extension that blocks WebRTC, or by switching to a browser that allows disabling it through a setting, such as Firefox (`about:config`, `media.peerconnection.enabled = false`).

For a future round: the balancer could gain `UDP ASSOCIATE` support, or the host could block outbound UDP outside the tunnel with a firewall rule, to drop WebRTC instead of letting it leak. The controller also hasn't been tested against real containers yet, only unit and integration tests against the real balancer.

## Host requirements

- Kernel with the WireGuard module loaded (`sudo modprobe wireguard`).
- Docker with Compose v2 (for the quick test) and access to the Docker socket (for the controller).
- Go, just to build the controller. The `go.mod` asks for a recent version (pulled in by the Docker SDK); with `GOTOOLCHAIN=auto` (the Go 1.21+ default), `go build` itself downloads the right toolchain if the installed one is older.

## License

To be defined. If code from the original project is reused, its BSD-2-Clause copyright notice must be kept.
