# VPS NAT Backend

VPS NAT Backend is a Go-based backend for running a Telegram-driven VPS selling platform on top of Incus.

It combines customer-facing Telegram flows, an internal admin API, wallet and payment handling, service lifecycle management, support ticketing, and infrastructure orchestration for NAT-based containers.

At a high level, the platform lets users:
- browse VPS packages from Telegram
- buy a VPS with wallet or QRIS
- manage their VPS from Telegram
- top up balance
- open support tickets

At the same time, it gives admins:
- package and user management
- container actions
- finance and server cost summaries
- wallet adjustments
- activity logs
- monitoring alerts

## What This Project Does

This backend is designed for a NAT VPS business model where:
- each customer service is an Incus container
- each container receives a private IP
- public access is exposed through forwarded ports on the node
- domain-based access can be provisioned through Caddy reverse proxy

The backend acts as the control plane for:
- business rules
- billing
- provisioning
- support workflows
- operational visibility

## Key Features

### Telegram customer flows
- Telegram user sync and wallet bootstrap
- package listing for `/buyvps`
- VPS purchase flow
  - wallet purchase
  - QRIS purchase via Pakasir
- `My VPS` actions
  - start
  - stop
  - reboot
  - change password
  - reset SSH password
  - renew
  - upgrade
  - reinstall
  - transfer
  - cancel
- domain setup preview and provisioning
- NAT port reconfiguration
- wallet top-up
- support ticket create/list/detail/reply

### Admin API
- admin login/logout with bearer sessions
- package CRUD
- user list and detail
- container list, detail, and actions
- support ticket management
- dashboard overview
- finance summary
- server cost history
- wallet adjustments
- monitoring alerts
- activity logs

### Infrastructure and operations
- Incus integration via the official Go client
- NAT port forwarding management
- QRIS payment verification with Pakasir webhook flow
- domain provisioning through Caddy Admin API
- background dashboard metrics cache
- CPU/RAM alert monitoring with Telegram admin notifications

## High-Level Architecture

The system is split into a few main pieces:

1. **Backend API**
- written in Go
- exposes Telegram-oriented endpoints and internal admin endpoints
- stores business state in PostgreSQL

2. **Incus**
- runs customer containers
- applies runtime lifecycle actions
- handles network forwards for NAT access

3. **Caddy**
- runs as a separate service, typically on the Incus node or another internal edge host
- receives reverse proxy configuration through the Caddy Admin API
- proxies domains to `private_ip:target_port` inside containers

4. **Pakasir**
- used for QRIS transaction creation and verification

5. **Telegram bot(s)**
- a customer-facing bot calls the `/telegram/...` endpoints
- an optional admin alert bot receives operational notifications

## Tech Stack

- **Language:** Go
- **HTTP framework:** Gin
- **ORM / DB access:** GORM
- **Database:** PostgreSQL
- **Container platform:** Incus
- **Reverse proxy:** Caddy
- **Payments:** Pakasir QRIS
- **Authentication:** bearer-token admin sessions

Main dependencies include:
- `github.com/gin-gonic/gin`
- `gorm.io/gorm`
- `gorm.io/driver/postgres`
- `github.com/lxc/incus/v6`
- `github.com/joho/godotenv`
- `golang.org/x/crypto`

## Repository Layout

```text
cmd/
  api/                Main HTTP server entrypoint
  admin-seed/         Helper CLI to create or update an admin user

database/             SQL migrations

internal/
  activitylog/        Shared activity log writer
  adminops/           Dashboard, finance, monitoring, alerts
  app/                Application bootstrap
  auth/               Admin authentication and sessions
  config/             Environment-based configuration
  containers/         Internal admin container APIs
  http/               Handlers, middleware, router, response envelope
  incus/              Incus client wrapper
  model/              GORM models
  packages/           Package management
  support/            Support ticketing
  telegram/           Telegram customer flows and provisioning logic
  users/              User listing and detail
```

## Requirements

Before running the project, make sure you have:

- Go `1.26.1` or compatible
- PostgreSQL
- an Incus server if you want provisioning and runtime actions
- Caddy if you want active domain provisioning
- Pakasir credentials if you want QRIS flows

## Environment Variables

Copy the example file first:

```bash
cp .env.example .env
```

Important variables:

### App and HTTP
- `APP_NAME`
- `APP_ENV`
- `HTTP_HOST`
- `HTTP_PORT`

### Database
- `DB_HOST`
- `DB_PORT`
- `DB_USER`
- `DB_PASSWORD`
- `DB_NAME`
- `DB_SSLMODE`
- `DB_TIMEZONE`

### Admin auth
- `AUTH_ADMIN_SESSION_TTL`

### Telegram
- `TELEGRAM_BOT_SECRET`

### Pakasir
- `PAKASIR_BASE_URL`
- `PAKASIR_PROJECT_SLUG`
- `PAKASIR_API_KEY`

### Admin alerts
- `ADMIN_ALERT_TELEGRAM_BOT_TOKEN`
- `ADMIN_ALERT_TELEGRAM_CHAT_ID`

### Caddy
- `CADDY_ADMIN_URL`
- `CADDY_ADMIN_API_TOKEN`

### Incus
- `INCUS_ENABLED`
- `INCUS_MODE`
- `INCUS_SOCKET`
- `INCUS_REMOTE_ADDR`
- `INCUS_NETWORK_NAME`
- `INCUS_USER_AGENT`
- `INCUS_TLS_CLIENT_CERT_PATH`
- `INCUS_TLS_CLIENT_KEY_PATH`
- `INCUS_TLS_CA_PATH`
- `INCUS_TLS_SERVER_CERT_PATH`
- `INCUS_TLS_INSECURE_SKIP_VERIFY`

## Local Development Setup

### 1. Create the database

Create a PostgreSQL database, for example:

```sql
CREATE DATABASE vps_nat;
```

### 2. Apply database migrations

This repository stores migrations in the [`database/`](/root/project/vps-nat/database) directory, but it does not currently bundle a migration runner command.

Use your preferred migration workflow to apply the SQL files in order:

```text
database/000001_...
database/000002_...
...
database/000026_...
```

You can do this with your existing tooling, a custom script, or manual `psql` execution in sequence.

### 3. Configure `.env`

At minimum for basic local boot:

```env
APP_ENV=development
HTTP_PORT=8080

DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=vps_nat
DB_SSLMODE=disable

AUTH_ADMIN_SESSION_TTL=24h
```

For Telegram endpoint authorization:

```env
TELEGRAM_BOT_SECRET=your-shared-secret
```

For Incus-backed provisioning:

```env
INCUS_ENABLED=true
INCUS_MODE=remote
INCUS_REMOTE_ADDR=https://your-incus-node:8443
INCUS_NETWORK_NAME=your-bridge-name
INCUS_TLS_CLIENT_CERT_PATH=/path/to/client.crt
INCUS_TLS_CLIENT_KEY_PATH=/path/to/client.key
INCUS_TLS_CA_PATH=/path/to/ca.crt
INCUS_TLS_SERVER_CERT_PATH=/path/to/server.crt
```

For active domain provisioning through Caddy:

```env
CADDY_ADMIN_URL=http://127.0.0.1:2019
CADDY_ADMIN_API_TOKEN=
```

### 4. Seed an admin user

Use the helper command:

```bash
go run ./cmd/admin-seed --email admin@example.com --password AdminPass123! --role super_admin
```

Supported roles:
- `super_admin`
- `admin`

### 5. Run the API

```bash
go run ./cmd/api
```

Health endpoints:
- `GET /health`
- `GET /healthz`

## Production Notes

### Incus

Provisioning and runtime actions depend on a working Incus connection. If `INCUS_ENABLED=false`, infrastructure-related features will be unavailable or partially degraded.

### Caddy

Caddy is expected to run as a separate service. In production, it is usually installed on the public-facing host or Incus node that receives domain traffic on ports `80` and `443`.

The backend does not embed Caddy. Instead, it configures Caddy through its Admin API using:
- `CADDY_ADMIN_URL`
- `CADDY_ADMIN_API_TOKEN`

### Pakasir

QRIS purchase and wallet top-up flows depend on Pakasir transaction creation and webhook verification.

### Telegram alerts

Admin alert notifications use a separate Telegram bot configuration:
- `ADMIN_ALERT_TELEGRAM_BOT_TOKEN`
- `ADMIN_ALERT_TELEGRAM_CHAT_ID`

## Main API Areas

### Telegram endpoints
- `/telegram/start`
- `/telegram/home`
- `/telegram/buy-vps`
- `/telegram/buy-vps/submit`
- `/telegram/buy-vps/status`
- `/telegram/wallet/topup/submit`
- `/telegram/wallet/topup/status`
- `/telegram/my-vps/*`
- `/telegram/support/*`

### Admin endpoints
- `/auth/*`
- `/packages`
- `/users`
- `/containers`
- `/support/tickets`
- `/dashboard/overview`
- `/activity-logs`
- `/finance/*`
- `/monitoring/alerts`
- `/wallet/adjustments`

### Payment webhook
- `/payments/pakasir/webhook`

## Testing

Run the full test suite:

```bash
go test ./...
```

Build the project:

```bash
go build ./...
```

## Current Status

This repository already includes:
- customer Telegram purchase and VPS management flows
- wallet top-up via QRIS
- support ticketing
- admin dashboard, finance, activity logs, and monitoring APIs
- Incus-backed provisioning and NAT operations
- Caddy-backed domain provisioning

Some infrastructure-heavy flows still depend on your real environment being available and correctly configured, especially:
- Incus runtime access
- network forward support
- Caddy Admin API access
- Pakasir webhook verification

## License

No license file is currently included in this repository.
