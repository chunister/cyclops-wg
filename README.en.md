# cyclops-wg

[Português](README.md) · [Español](README.es.md)

WireGuard gateways with a SOCKS5 entry point and runtime exit rotation.

A single SOCKS5 entry point spreads connections across several containers. Each container keeps a WireGuard tunnel to a different server. You can change servers without rebuilding the image and without stopping the stack.

## Origin

The idea comes from [tatu-rotate](https://github.com/marciosouza20/tatu-rotate), which uses Surfshark OpenVPN with Privoxy and Mubeng. This project changes the architecture: it uses WireGuard, native SOCKS5, and runtime control.

## Status

Under construction. There is no working code yet.

## Plan

- Gateway: one container per exit, with WireGuard, microsocks, and an exit IP check.
- SOCKS5 balancer: receives connections on the entry port and picks a healthy gateway for each one.
- Control API: changes a gateway's server without a rebuild, draining connections before the change.
- Configuration: `.env.example` is versioned. The `.env` file and the WireGuard `.conf` files stay out of git.

## Stack

- Language: Go. The balancer compiles to a static binary.
- Gateway: Debian 13 slim with `wireguard-tools` and `microsocks`.
- Balancer: its own SOCKS5 server (no authentication, CONNECT only). It accepts connections from any SOCKS5 client, such as Burp or curl, and picks a healthy gateway per connection, round-robin. It tests each gateway every `HEALTH_INTERVAL` seconds and drops one from rotation after `HEALTH_FAILS` failures in a row. It exposes `GET /status` with each gateway's state.

## Setup

1. Copy `.env.example` to `.env`.
2. Generate a WireGuard key in the Surfshark dashboard and fill in `WG_PRIVATE_KEY`.
3. List the servers you want in `SERVERS`.

Full documentation on generating keys and `.conf` files will come once the gateway is ready.

## Known limitations

The balancer only implements the SOCKS5 `CONNECT` command, which is TCP. The browser's WebRTC uses STUN over UDP to discover the public IP, and that traffic doesn't go through the tunnel: it leaves directly over the internet, revealing the real user IP even with the proxy configured.

Tested on [ipleak.net](https://ipleak.net): the HTTP exit IP and DNS both showed the correct WireGuard server, with no leak. The WebRTC test showed the real ISP IP.

Mitigation available today: disable WebRTC in the browser you use with cyclops-wg. Chromium has no native toggle for this; it works with an extension that blocks WebRTC, or by switching to a browser that allows disabling it through a setting, such as Firefox (`about:config`, `media.peerconnection.enabled = false`).

For a future round: the balancer could gain `UDP ASSOCIATE` support, or the host could block outbound UDP outside the tunnel with a firewall rule, to drop WebRTC instead of letting it leak.

## Host requirements

- Kernel with the WireGuard module loaded (`sudo modprobe wireguard`).
- Docker with Compose v2.

## License

To be defined. If code from the original project is reused, its BSD-2-Clause copyright notice must be kept.
