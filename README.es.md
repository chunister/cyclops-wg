# cyclops-wg

[Português](README.md) · [English](README.en.md)

Gateways WireGuard con entrada SOCKS5 y rotación de salida en tiempo de ejecución.

Un único punto de entrada SOCKS5 reparte las conexiones entre varios contenedores. Cada contenedor mantiene un túnel WireGuard hacia un servidor distinto. Puedes cambiar los servidores sin reconstruir la imagen y sin detener el stack.

## Origen

La idea surgió del proyecto [tatu-rotate](https://github.com/marciosouza20/tatu-rotate), que usa OpenVPN de Surfshark con Privoxy y Mubeng. Este proyecto cambia la arquitectura: usa WireGuard, SOCKS5 nativo y control en tiempo de ejecución.

## Estado

En construcción. Todavía no hay código funcional.

## Plan

- Gateway: un contenedor por salida, con WireGuard, microsocks y una comprobación de la IP de salida.
- Balanceador SOCKS5: recibe las conexiones en el puerto de entrada y elige un gateway sano para cada una.
- API de control: cambia el servidor de un gateway sin reconstruir, drenando las conexiones antes del cambio.
- Configuración: `.env.example` se versiona. El `.env` y los archivos `.conf` de WireGuard quedan fuera de git.

## Stack

- Lenguaje: Go. El balanceador y la API de control se compilan en un binario estático.
- Gateway: Debian slim con `wireguard-tools` y `microsocks`.

## Configuración

1. Copia `.env.example` a `.env`.
2. Genera la clave WireGuard en el panel de Surfshark y completa `WG_PRIVATE_KEY`.
3. Indica los servidores que quieras en `SERVERS`.

La documentación completa sobre cómo generar las claves y los archivos `.conf` llegará cuando el gateway esté listo.

## Requisitos del host

- Kernel con el módulo WireGuard cargado (`sudo modprobe wireguard`).
- Docker con Compose v2.

## Licencia

Por definir. Si se reutiliza código del proyecto original, hay que mantener el aviso de copyright que exige su licencia BSD-2-Clause.
