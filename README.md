# 💳 Microservices Fintech Ecosystem

A robust, developer-first microservices platform for financial operations, featuring a secure API Gateway, distributed services, and a Stripe-inspired CLI for seamless local development.

## 🚀 Architecture

The system is a professional event-driven ecosystem built with **Go**, **gRPC**, **Kafka**, **RabbitMQ**, **PostgreSQL**, and **Redis**.

```mermaid
graph TD
    User((User/Dev)) -->|HTTP/WS| Gateway[API Gateway :8080]
    
    subgraph "Internal Communication (gRPC)"
        Gateway -->|gRPC: ValidateKey| AuthService[Auth Service :8081]
        Gateway -->|Proxy: HTTP| PaymentsService[Payments Service :8082]
        PaymentsService -->|gRPC: RecordTransaction| LedgerService[Ledger Service :8083]
    end

    subgraph "Event-Driven Backbone"
        PaymentsService -->|Produce: payment.succeeded| Kafka[(Kafka/Redpanda)]
        Kafka -->|Consume: Event Sourcing| LedgerService
        
        PaymentsService -->|Queue: NotifyTask| RabbitMQ[(RabbitMQ)]
        RabbitMQ -->|Consume: Worker| NotificationsService[Notifications Service]
    end
    
    subgraph "Real-time Relay"
        PaymentsService -->|Publish| Redis[(Redis Pub/Sub)]
        Redis -->|Stream| Gateway
        Gateway -->|WebSocket| CLI[Micro CLI]
    end
```

- **API Gateway**: Central entry point. Uses **gRPC** for high-performance internal auth validation and handles WebSocket-based webhook streaming.
- **Auth Service**: Manages user identity and API keys. Hosts a **gRPC server** for internal key validation.
- **Payments Service**: Orchestrates payment processing. Acts as an event producer for Kafka and a task producer for RabbitMQ.
- **Ledger Service**: Robust event-sourced accounting system. Consumes from **Kafka** for persistent audit trails and provides high-speed **gRPC** access for real-time balance checks.
- **Notifications Service**: Async worker that handles email and SMS delivery via **RabbitMQ** tasks.
- **Micro CLI**: Professional tool for local developer productivity and webhook testing.

## 🛠️ Tech Stack

- **Languge**: Go 1.24+
- **Internal APIs**: gRPC & Protocol Buffers
- **Event Sourcing**: Kafka (via Redpanda)
- **Async Workers**: RabbitMQ
- **Databases**: PostgreSQL (Isolated per service)
- **Caching/Real-time**: Redis
- **Infrastructure**: Docker & Docker Compose
- **CLI Framework**: Cobra & Viper

---

## 📦 Getting Started

### 1. Launch the Ecosystem

```bash
docker-compose up --build -d
```
The Gateway is exposed at `http://localhost:8080`.

### 2. Install Micro CLI

```bash
go build -o micro ./cmd/cli
```

### 3. Developer Onboarding

```bash
# Register a new developer account
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "dev@example.com", "password": "securepassword"}'

# Login via CLI
./micro login
```

---

## 📖 API Reference

### 🔐 Auth Service
Requires session JWT for key management.
- **gRPC Port**: `:50051` (Internal Validation)
- **HTTP Port**: `:8081` (Management)

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/auth/register` | `POST` | Create a new developer account |
| `/auth/login` | `POST` | Authenticate and receive JWT |
| `/auth/api_keys` | `POST` | Generate a new API key (`test` or `live`) |

### 💸 Payments Service
Requires `Authorization: Bearer sk_<key>` header.
- **Port**: `:8082`

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/payments/payment_intents` | `POST` | Create a new payment intent |
| `/payments/payment_intents/{id}/confirm` | `POST` | Confirm a payment intent |

### 📒 Ledger Service
Consumes events from Kafka and provides state via gRPC.
- **gRPC Port**: `:50052`
- **HTTP Port**: `:8083`

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/ledger/accounts` | `POST` | Create a financial account |
| `/ledger/accounts/{id}` | `GET` | View account balance and type |
| `/ledger/transactions` | `POST` | Record a double-entry transaction |

---

## ⚡ Real-time Webhooks

Test local integrations without tunnels:

1. **Listen for events**:
   ```bash
   ./micro listen --forward-to http://localhost:4242/webhook
   ```
2. **Workflow**: 
   - Payment confirmed -> Event published to **Redis** (Real-time Relay) & **Kafka** (Persistent Audit).
   - Gateway relays Redis pub/sub to the CLI via WebSocket.

---

## 💻 Developer Guide

### Makefile Commands
- `make build`: Build all services locally.
- `make proto`: Re-generate Go code from Protobuf definitions.
- `make test`: Run the test suite.
- `make clean`: Remove binaries and temporary files.

### Database Access
Connect via `psql` (passwords are in `docker-compose.yml`):
- `psql postgres://user:password@localhost:5433/microservices` (Auth)
- `psql postgres://user:password@localhost:5434/payments` (Payments)
- `psql postgres://user:password@localhost:5435/ledger` (Ledger)

### Infrastructure Access
- **RabbitMQ Management**: `http://localhost:15672` (User: `user`, Pass: `password`)
- **Redpanda Console**: (Available if console is added to compose)

---

## 📜 License
MIT
