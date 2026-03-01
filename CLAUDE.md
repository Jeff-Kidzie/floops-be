# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the server
go run ./cmd/api/main.go

# Build
go build -o floops-be ./cmd/api/main.go

# Run tests
go test ./...

# Run a single test
go test ./internal/handlers/ -run TestRegister

# Lint (requires golangci-lint)
golangci-lint run

# Hot reload (requires air)
air
```

## Environment

The app loads from `local.env` at startup (uses `export KEY=VALUE` format). Required variables:

```
DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
```

`local.env` is gitignored (matches `*.env`). The `tmp/` directory is used by `air` for hot-reload build artifacts.

## Architecture

**Entry point:** `cmd/api/main.go` — initializes the DB connection (via `loadDb()`), then sets up Gin routes.

**Package layout:**
- `internal/database/` — global `*sql.DB` singleton (`database.DB`), connected via `lib/pq` to PostgreSQL
- `internal/handlers/` — Gin handler functions; currently `auth.go` with `Register`, `Login`, `ForgotPassword`
- `internal/models/` — plain Go structs; `User` has `PasswordHash json:"-"` to prevent serialization

**Auth flow:** Handlers use raw `database/sql` queries directly (no ORM). Passwords are hashed with bcrypt. JWT signing uses `golang-jwt/jwt/v5`; the secret is currently hardcoded in `auth.go` (`jwtSecret`).

**Routes:**
- `GET /health`, `GET /db-health`
- `POST /auth/register`, `POST /auth/login`, `POST /auth/forgot-password`

**Known TODOs in `internal/handlers/auth.go`:** Login handler completes bcrypt comparison but does not yet generate or return a JWT token.
