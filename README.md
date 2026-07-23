# VeritasVPN

> Privacy is truth. WireGuard-only, open-source, no-logs VPN service.

## Architecture

```
┌─────────────┐  ┌──────────────┐  ┌────────────────────┐
│ Client Apps │──│ API Gateway  │──│ Auth/Provisioning  │
│ (Desktop/   │  │ (gRPC/REST) │  │                     │
│  Mobile/CLI)│  └──────────────┘  └────────────────────┘
└─────────────┘         │                    │
       ▲                ▼                    ▼
       │        ┌──────────────┐  ┌────────────────────┐
       │        │ WireGuard    │  │ PostgreSQL + Redis │
       └────────│ Servers      │  │ + NATS             │
                └──────────────┘  └────────────────────┘
```

## Quick Start

```bash
make dev-up
```

Services:
- **auth-svc** → `:8081` — Account registration, JWT auth
- **wg-manager** → `:8082` — WireGuard peer and server management
- **billing-svc** → `:8083` — Subscription and payment processing
- **PostgreSQL** → `:5432`
- **Redis** → `:6379`
- **NATS** → `:4222`

## Building

```bash
make build-all    # Build all services
make build-cli    # Build CLI client
make test         # Run tests
```

## CLI Usage

```bash
veritas register                # Create anonymous account
veritas servers                 # List available servers
veritas connect --region eu     # Connect to VPN
veritas status                  # Show active connections
veritas disconnect              # Disconnect
```

## Project Structure

```
├── api/proto/       # Protobuf definitions
├── lib/             # Shared Go libraries
│   ├── config/      # Config loading from env
│   ├── logging/     # Structured logging (zap)
│   ├── crypto/      # Token generation, hashing
│   └── jwt/         # JWT creation/validation
├── services/
│   ├── auth-svc/    # Auth service (gRPC)
│   ├── wg-manager/  # WireGuard manager (gRPC)
│   ├── billing-svc/ # Billing service (REST)
│   └── veritas-agent/ # Per-server agent
├── clients/
│   ├── cli/         # CLI client
│   ├── desktop/     # Tauri 2 + React (planned)
│   └── mobile/      # Flutter (planned)
├── infra/
│   ├── terraform/   # Infrastructure as code
│   └── ansible/     # Server provisioning
└── docker-compose.yml
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22+ |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Event Bus | NATS |
| VPN Protocol | WireGuard |
| Desktop | Tauri 2 + React (planned) |
| Mobile | Flutter (planned) |
| Monitoring | Prometheus + Grafana |

## License

Source available. Proprietary components are BSL licensed.
