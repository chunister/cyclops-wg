# 👁️ cyclops-wg

[Português](README.md) · [Español](README.es.md)

WireGuard gateways with a SOCKS5 entry point and runtime exit rotation.

A single SOCKS5 entry point spreads connections across several containers. Each container keeps a WireGuard tunnel to a different server. You can change servers without rebuilding the image and without stopping the stack.

## 🌱 Origin

The idea comes from [tatu-rotate](https://github.com/marciosouza20/tatu-rotate), which uses Surfshark OpenVPN with Privoxy and Mubeng. This project changes the architecture: it uses WireGuard, native SOCKS5, and runtime control.

## 📋 Host requirements

On Debian 13, current Kali GNU/Linux, or Ubuntu, `sudo scripts/install-deps.sh` installs all of this at once (Docker, Compose v2, Go, and the WireGuard module). On another distro, or if you'd rather install by hand, here's the list:

- A Linux kernel with the WireGuard module loaded (`sudo modprobe wireguard`). `wireguard-tools` (`wg`) itself is only used inside the gateway container, installed by its own `Dockerfile`. No need to install it on the host, but the [install page](https://www.wireguard.com/install/) is there if you want to try it outside the project. Tested on Kali GNU/Linux 2025.4, kernel `6.16.8+kali-amd64`.
- [Docker Engine](https://docs.docker.com/engine/install/) 27 or newer, with Compose v2 (for the quick test) and access to the Docker socket (for the controller). Tested with Docker `27.5.1+dfsg4` and Docker Compose `v2.32.4-3`.
- [Go](https://go.dev/doc/install), just to build the controller. The `go.mod` asks for Go 1.26 (pulled in by the Docker SDK); with `GOTOOLCHAIN=auto` (the Go 1.21+ default), `go build` itself downloads the right toolchain if the installed one is older. Tested with `go1.26.3`.
- A [Surfshark](https://surfshark.com/) account, to generate the WireGuard key used in `.env` (see "Setup" below). To sign up with a referral bonus, use [this link](https://surfshark.club/friend/adjVkuXB) ("You got a gift from a friend").

## ✅ Status

Tested end to end on a Kali box and on a Debian 13 box: a real WireGuard tunnel, the SOCKS5 balancer working with curl and with Burp, server switching without stopping the container, and the controller bringing up the full set (10 active gateways plus 3 spares) against a real Docker daemon. Automatic promotion was also tested by killing an active gateway on purpose:

```bash
docker kill cyclops-gw-1
```

A spare took its place, and the killed gateway was restarted and pointed at a different server in the background.

## 🧩 Pieces

- **Gateway** (`gateway/`): one container per exit, with WireGuard, microsocks, and an exit-IP check via `healthcheck.sh`. `switch-server.sh`, run with `docker exec`, changes the tunnel's server without restarting the container.
- **Balancer** (`balancer/`): its own SOCKS5 server in Go (no authentication, CONNECT only). It accepts connections from any SOCKS5 client, such as Burp or curl, and picks a healthy gateway per connection, round-robin. It tests each gateway every `HEALTH_INTERVAL` seconds and drops one from rotation after `HEALTH_FAILS` failures in a row. It exposes `GET /status` with each gateway's state, and `PUT /gateways` to swap the active list without restarting.
- **Controller** (`controller/`): runs directly on the host, not in a container, and talks to Docker through its official API. It brings up `GATEWAY_COUNT` active gateways plus `SPARE_COUNT` spares, already tunneled. When an active gateway keeps failing, it promotes a healthy spare in its place, updates the balancer via `PUT /gateways`, and switches the failed one's server in the background, respecting a cooldown (`SERVER_COOLDOWN`) before that server is reused. With `LATENCY_TOP_SERVERS` on, that server choice (at boot, on a promotion, or on a rotation) stays limited to the N with the lowest average ping: the controller pings each catalog server's `Endpoint` (no tunnel needed), keeps a moving average in `LATENCY_FILE`, and keeps it fresh every `LATENCY_CHECK_INTERVAL` seconds.

## 🛠️ Stack

- Language: Go, for both the balancer and the controller.
- Gateway: Debian 13 slim with `wireguard-tools` and `microsocks`.
- Orchestration: the Docker API in Go (`github.com/docker/docker/client`), not `docker compose`, because the controller needs to create and inspect containers dynamically.

## ⚙️ Setup

1. Copy `.env.example` to `.env`.
2. Generate a WireGuard key in the Surfshark dashboard and fill in `WG_PRIVATE_KEY` in `.env`. `WG_ADDRESS` already has the value Surfshark usually assigns (`10.14.0.2/16`); check it against your downloaded `.conf`'s `Address` and only change it if it's different.
3. The servers in `servers/` already come ready in the repo. Only download new `.conf` files and run `scripts/extract-peer.sh` if you want to add servers beyond the ones already there.

### ⚡ Quick test with 1 gateway

Also fill in `GATEWAY1_SERVER` in `.env`, with the name of a file in `servers/` without the extension. Then:

```bash
docker compose up --build -d
curl -s --socks5-hostname 127.0.0.1:1080 http://ip-api.com/json
```

### 🚀 Full run, with automatic spares

Needs `GATEWAY_COUNT + SPARE_COUNT` distinct servers in `servers/` (10 + 3 by default). Load the WireGuard module on the host (`sudo modprobe wireguard`), export the variables from `.env`, and run the controller:

```bash
cd controller
go build -o controller .
set -a && source ../.env && set +a
sudo -E ./controller
```

The controller needs enough privilege to talk to Docker (or your user in the `docker` group), and manages containers directly, without `docker-compose.yml`.

By default, the terminal where the controller runs turns into a live dashboard: a summary at the top (how many active and spare gateways are healthy, how long it's been running), a host CPU/RAM usage panel, plus the proxy's current upload and download (measured in the balancer itself, via `GET /throughput`), two side-by-side tables with the active and spare gateways (server, internal IP, WireGuard exit IP, latest health check's ping colored by range (green, yellow, red), time since the server last changed, failures in a row, state, which also turns "degraded" when the connection is fine but slow), a table with the balancer's most recent connections (time, client origin, destination, which gateway served it, and its exit IP, up to `CONN_HISTORY` rows), and an events panel at the bottom with the same log that used to go straight to the terminal. Shortcuts: `q` or `Ctrl-C` opens a confirmation with three options: just quit (the containers keep running and the proxy stays up), quit and remove all containers, or cancel; `r` asks for confirmation (defaults to "no") before triggering a full rotation (switches every gateway's server at once, even the ones that haven't failed); the same rotation can also run on its own every `FULL_ROTATE_INTERVAL` seconds (0 disables it, the default); `p` pauses and resumes the screen's updates (the containers keep running as usual); `c` clears the events panel; `l`/`L` switches the screen's language forward/back (English, Portuguese, Spanish), same as `DASH_LANG` sets at startup (`en` by default); `t`/`T` switches the dashboard's color theme forward/back, among 10 themes (the original look, Dracula, Nord, Solarized Dark, Gruvbox Dark, high contrast, Monokai, One Dark, Tokyo Night, and Catppuccin Mocha). The current language and theme show up in the top-right corner of the summary's first line, and get saved to `PREFS_FILE` (`./dashboard-prefs.json` by default) to reopen in the same state next run; after the first keyboard change, that file takes precedence over `DASH_LANG`. Besides the screen, the same log is also written to `LOG_FILE` (`./controller.log` by default, `""` disables it), useful for reviewing after a load test, when the events panel (only keeps the last 500 lines) or the terminal get hard to follow live. To run without this screen (for example behind `nohup` or a systemd service, with no interactive terminal), set `DASHBOARD=0` in `.env` and watch `curl -s http://127.0.0.1:9090/status` instead.

## ⚠️ Known limitations

The balancer only implements the SOCKS5 `CONNECT` command, which is TCP. The browser's WebRTC uses STUN over UDP to discover the public IP, and that traffic doesn't go through the tunnel: it leaves directly over the internet, revealing the real user IP even with the proxy configured.

Tested on [ipleak.net](https://ipleak.net): the HTTP exit IP and DNS both showed the correct WireGuard server, with no leak. The WebRTC test showed the real ISP IP.

Mitigation available today: disable WebRTC in the browser you use with cyclops-wg. Chromium has no native toggle for this; it works with an extension that blocks WebRTC, or by switching to a browser that allows disabling it through a setting, such as Firefox (`about:config`, `media.peerconnection.enabled = false`).

For a future round: the balancer could gain `UDP ASSOCIATE` support, or the host could block outbound UDP outside the tunnel with a firewall rule, to drop WebRTC instead of letting it leak.

## 📄 License

[GPLv3](LICENSE). If code from the original project is reused, its BSD-2-Clause copyright notice must be kept.
