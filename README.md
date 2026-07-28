# VeritasVPN

> Privacy is truth. WireGuard-only VPN (SOCKS only where WireGuard cannot run, e.g. Chrome).

## Architecture

```
Desktop / CLI  --WireGuard-->  linux node wg0 (veritas-agent)
                     ^
Website / API  -->  nginx --> auth-svc, billing-svc, wg-manager
                                      |
                               SSE peer updates --> agent
```

The **VPN server is the Linux node** (today `linuxDesktop`, later a VPS). Your Mac is a client only.

## Quick Start (VPN node)

```bash
sudo bash deploy/node/bootstrap-wg.sh   # optional if agent brings up wg0 itself
# set PUBLIC_IP in .env to this host's public IP
docker compose up -d --build
```

Services:
- **auth-svc** → `:8081`
- **wg-manager** → `:8082` — peer provisioning + agent API
- **billing-svc** → `:8083`
- **veritas-agent** — host WireGuard (`wg0`, UDP `51820`)
- **veritas-proxy** → `:1080` — SOCKS for Chrome extension only
- **nginx** → `:8000`

Forward **UDP 51820** on the router for remote clients.

## Desktop

Connect uses WireGuard against the node. If `wg`/`wg-quick` are missing, it falls back to SOCKS.

## CLI

```bash
export VERITAS_API_URL=https://veritasvpn.cloud/api/v1
export VERITAS_ACCESS_TOKEN=...
veritas connect
veritas disconnect
```

## Building

```bash
make build-all
make test
```

## License

Source available. Proprietary components are BSL licensed.
