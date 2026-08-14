# raddigo

A small, production-style REST API written in Go with Gin and a clean, layered
architecture.

## Features

- User registration with email verification
- One-to-many `User` → `Address` relationship
- Passwords hashed with bcrypt
- Layered design: `handler` → `service` → `repository`
- PostgreSQL storage via GORM (swappable through the `UserRepository` interface)
- Automatic schema migration on startup (GORM `AutoMigrate`)
- Structured JSON logging with `log/slog`
- Request logging and panic-recovery middleware
- Graceful shutdown on `SIGINT` / `SIGTERM`
- Environment-based configuration

## Project layout

```
cmd/server            application entrypoint
internal/config       environment configuration
internal/model        domain types
internal/database     GORM connection + AutoMigrate
internal/repository   persistence layer + interface
internal/mailer       email delivery (dev log mailer)
internal/service      business logic + validation
internal/handler      HTTP handlers + JSON helpers
internal/middleware   logging & recovery middleware
internal/server       router and HTTP server lifecycle
```

## Requirements

- Go 1.22 or newer (developed against Go 1.26)
- PostgreSQL 12+

## Getting started

Start PostgreSQL (e.g. via Docker):

```bash
docker run --name postgres-db \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=raddigo \
  -p 5432:5432 -d postgres:latest
```

Then build and run:

```bash
# fetch dependencies
go mod tidy

# run the server (creates the tables automatically)
make run          # or: go run ./cmd/server

# run tests
make test

# build a binary into ./bin
make build
```

The server listens on `:8080` and connects to
`postgres://postgres:postgres@localhost:5432/raddigo?sslmode=disable` by
default. Override via environment variables (see `.env.example`), including
`DATABASE_URL` and `APP_BASE_URL` (used to build verification links).

> Email sending is stubbed by a development log mailer: the verification link is
> written to the application logs instead of being emailed.

## API

Base path: `/api/v1`

| Method | Path                    | Description                     |
| ------ | ----------------------- | ------------------------------- |
| GET    | `/healthz`              | Health check                    |
| POST   | `/api/v1/auth/register` | Register a user + addresses     |
| GET    | `/api/v1/auth/verify`   | Verify email via `?token=`      |

### Examples

```bash
# register
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "first_name": "Jane",
    "last_name": "Doe",
    "email": "jane@example.com",
    "mobile_no": "1234567890",
    "password": "supersecret",
    "addresses": [
      {
        "address1": "1 Main St",
        "address2": "Apt 2",
        "street": "Main",
        "city": "Metropolis",
        "state": "NY",
        "country": "US"
      }
    ]
  }'

# verify (token is printed to the server logs by the dev mailer)
curl -s 'localhost:8080/api/v1/auth/verify?token=<token>'
```
