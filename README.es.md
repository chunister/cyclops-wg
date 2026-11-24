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

- Lenguaje: Go. El balanceador se compila en un binario estático.
- Gateway: Debian 13 slim con `wireguard-tools` y `microsocks`.
- Balanceador: servidor SOCKS5 propio (sin autenticación, solo el comando CONNECT). Acepta conexiones de cualquier cliente SOCKS5, como Burp o curl, y elige un gateway sano por conexión, en round-robin. Prueba cada gateway cada `HEALTH_INTERVAL` segundos y saca de la rotación al que falle `HEALTH_FAILS` veces seguidas. Expone `GET /status` con el estado de cada gateway.

## Configuración

1. Copia `.env.example` a `.env`.
2. Genera la clave WireGuard en el panel de Surfshark y completa `WG_PRIVATE_KEY`.
3. Indica los servidores que quieras en `SERVERS`.

La documentación completa sobre cómo generar las claves y los archivos `.conf` llegará cuando el gateway esté listo.

## Limitaciones conocidas

El balanceador solo implementa el comando `CONNECT` de SOCKS5, que es TCP. El WebRTC del navegador usa STUN sobre UDP para descubrir la IP pública, y ese tráfico no pasa por el túnel: sale directo a internet, revelando la IP real del usuario aunque el proxy esté configurado.

Probado en [ipleak.net](https://ipleak.net): la IP de salida HTTP y el DNS mostraron el servidor WireGuard correcto, sin fuga. La prueba de WebRTC mostró la IP real del proveedor de internet.

Mitigación disponible hoy: desactiva el WebRTC en el navegador que uses con cyclops-wg. Chromium no tiene un botón nativo para esto; funciona con una extensión que bloquea WebRTC, o cambiando a un navegador que permita desactivarlo por configuración, como Firefox (`about:config`, `media.peerconnection.enabled = false`).

Para una próxima ronda: el balanceador podría ganar soporte para `UDP ASSOCIATE`, o el host podría bloquear el UDP de salida fuera del túnel con una regla de firewall, para tumbar el WebRTC en lugar de dejarlo filtrar.

## Requisitos del host

- Kernel con el módulo WireGuard cargado (`sudo modprobe wireguard`).
- Docker con Compose v2.

## Licencia

Por definir. Si se reutiliza código del proyecto original, hay que mantener el aviso de copyright que exige su licencia BSD-2-Clause.
