# VeritasVPN — Kubernetes Deployment

k3s-based deployment for VeritasVPN backend services and BTCPay Server.

## Prerequisites

- k3s cluster (single or multi-node)
- `kubectl` configured
- Docker (for building service images)

## Quick Start

```bash
# Build and push all service images to local registry
REGISTRY=localhost:31500 TAG=latest bash deploy/k8s/scripts/push-images.sh

# Apply the dev overlay
bash deploy/k8s/scripts/apply.sh dev

# Check status
make k8s-status
```

## Architecture

```
Namespace: veritas                      Namespace: btcpay
───────────────                         ───────────────
postgres (STS)          bitcoind (STS)  postgres-btcpay (STS)
redis (Deploy)          nginx (Deploy)  bitcoind (STS)
nats (STS)              ingress         nbxplorer (Deploy)
auth-svc (Deploy)       veritas-agent   btcpayserver (Deploy)
wg-manager (Deploy)     (DaemonSet)
billing-svc (Deploy)    veritas-proxy
registry (Deploy)       (Deploy)
```

## Namespaces

| Namespace | Purpose |
|-----------|---------|
| `veritas` | Core backend services (auth, WG manager, billing, nginx, proxy) |
| `btcpay` | BTCPay Server stack (bitcoind, nbxplorer, btcpayserver, postgres) |
| `ingress-nginx` | Ingress controller and Cloudflare tunnel |

## Overlays

| Overlay | Description |
|---------|-------------|
| `dev` | amd64 node selector, mock BTCPay enabled, debug logging |
| `prod` | production env, info logging, mock BTCPay disabled, Redis x2 replicas |

```bash
kubectl apply -k deploy/k8s/overlays/dev/
kubectl apply -k deploy/k8s/overlays/prod/
```

## BTCPay Server

```bash
kubectl apply -k deploy/k8s/btcpay/
```

BTCPay runs in its own namespace with testnet bitcoin by default. The billing-svc
reaches it at `btcpayserver.btcpay.svc.cluster.local:49392`.

To switch to mainnet, edit `deploy/k8s/btcpay/bitcoind.yaml`:
- `BITCOIN_NETWORK: "mainnet"`
- Ports: `8332` (RPC), `8333` (P2P)
- Update nbxplorer and btcpayserver env vars accordingly.

## veritas-agent DaemonSet

The agent runs only on nodes labeled `veritas-vpn-node=true`:

```bash
kubectl label node <node-name> veritas-vpn-node=true
```

It requires:
- WireGuard kernel module
- `/opt/veritasvpn/data/wireguard/private.key` (hostPath)
- hostNetwork + privileged

## Cloudflare Tunnel

The cloudflared deployment in `ingress-nginx` namespace tunnels traffic to the
ingress-nginx controller. To add the Veritas hostname:

1. In Cloudflare Zero Trust dashboard, edit your tunnel configuration
2. Add a public hostname: `veritasvpn.cloud` → `http://ingress-nginx-controller.ingress-nginx.svc.cluster.local:80`
3. Repeat for `www.veritasvpn.cloud`

## Building Images

```bash
# All service images
REGISTRY=localhost:31500 bash deploy/k8s/scripts/push-images.sh

# Single service
docker build -t localhost:31500/auth-svc:latest -f services/auth-svc/Dockerfile .
docker push localhost:31500/auth-svc:latest
```

## Scripts

| Script | Purpose |
|--------|---------|
| `scripts/apply.sh [dev|prod]` | Apply a Kustomize overlay |
| `scripts/push-images.sh` | Build and push all service images |
