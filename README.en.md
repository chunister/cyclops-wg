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

- Language: Go. The balancer and the control API compile to a static binary.
- Gateway: Debian slim with `wireguard-tools` and `microsocks`.

## Setup

1. Copy `.env.example` to `.env`.
2. Generate a WireGuard key in the Surfshark dashboard and fill in `WG_PRIVATE_KEY`.
3. List the servers you want in `SERVERS`.

Full documentation on generating keys and `.conf` files will come once the gateway is ready.

## Host requirements

- Kernel with the WireGuard module loaded (`sudo modprobe wireguard`).
- Docker with Compose v2.

## License

To be defined. If code from the original project is reused, its BSD-2-Clause copyright notice must be kept.
