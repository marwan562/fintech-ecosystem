# Product Roadmap

Strategic plan to evolve Sapliy into a **production-grade, open-source fintech automation platform** — combining the reliability of Stripe with the flexibility of Zapier.

---

## Vision

**Event-driven automation & policy platform for fintech and business flows.**

Provide developer-first, scalable infrastructure where:
- Everything is an **event**
- Events trigger **flows**
- Flows execute **actions**
- All within isolated **zones**

---

## Core Mental Model

```
Organization → Zone → Event → Flow → Action
```

| Concept | Purpose |
|---------|---------|
| **Organization** | Owns everything, has users/teams/policies |
| **Zone** | Isolated automation space with test/live modes |
| **Event** | The universal trigger (SDK, UI, providers) |
| **Flow** | Automation logic connecting events to actions |

---

## Versioned Journey

| Version | Focus | Outcome |
|---------|-------|---------|
| **v0.x** | Foundation | Core primitives (payments, ledger, webhooks) |
| **v1.0** | Quality | Tests, idempotency, clean layering |
| **v1.x** | Growth | SDKs, observability, wallets |
| **v2.x** | Services | Hosted version, enterprise support |
| **v3.x** | DX | SDK enhancement, performance |
| **v4.x** | Ecosystem | UI components, CLI v2, examples |
| **v5.x** | Automation | Zone platform, flow engine, policies |

---

## Completed Phases

### Foundation (v0.x) ✅
- [x] Core Primitives: Payments, Ledger, Webhooks
- [x] Infrastructure: Kubernetes, Docker Compose, CI/CD
- [x] Security: API Key hashing, OAuth2/OIDC, Scopes

### Quality & Credibility (v1.0) ✅
- [x] Unit Tests for Services
- [x] Table-Driven Tests
- [x] Mock Interfaces
- [x] Idempotency Keys
- [x] Ledger-Only Balance Updates
- [x] Layered Separation

### Growth (v1.x) ✅
- [x] SDKs (Node, Python, Go)
- [x] Advanced Observability
- [x] Wallets as First-Class Primitive
- [x] Subscriptions & Billing
- [x] Multi-tenancy & Rate Limiting

### Services (v2.x) ✅
- [x] Hosted Version (Fintech Cloud)
- [x] Enterprise Compliance
- [x] Paid Support & SLAs
- [x] Custom Integrations

---

## Current Phase

### Developer Experience (v3.x) ✅
- [x] **Complete SDK Coverage** — All APIs (Auth, Zone, Flow, Ledger, Payments) in all SDKs
- [x] **Comprehensive Examples** — Real-world Checkout, Audit, and Bridge flows
- [x] **SDK Publishing** — Pipeline ready for npm, PyPI, Go modules
- [x] **OpenAPI-based Generation** — Fully automated CI pipeline
- [x] **Advanced Caching** — Redis integration for Ledger and Zone services
- [x] **Batch Operations** — Bulk APIs for Zone, Flow, and Ledger

### Ecosystem Packages (v4.x) ✅
- [x] **@sapliyio/fintech-ui** — React components
- [x] **fintech-testing** — Test utilities
- [x] **sapliy-cli v2** — Enhanced CLI
- [x] **fintech-examples** — Sample apps
- [x] **Documentation Site** — VitePress docs

### Zone & Automation Platform (v5.x) ✅

The next major evolution — transforming from a payment processor into a full automation platform.

#### Core Zone Features
- [x] **Zone Management API** — CRUD for zones
- [x] **Test/Live Mode Isolation** — Separate keys, logs, flows
- [x] **Zone-Scoped Events** — Events bound to zones
- [x] **Zone Templates** — Quick-start configurations

#### Flow Engine
- [x] **Visual Flow Builder** — Drag-and-drop UI
- [x] **Event Triggers** — SDK, webhooks, schedule
- [x] **Logic Nodes** — Conditions, filters, approvals
- [x] **Action Nodes** — Webhooks, notifications, audit

#### Policy Engine
- [x] **Phase 1**: Hardcoded policies (admin, finance roles)
- [x] **Phase 2**: JSON policy language
- [x] **Phase 3**: Full OPA-style engine

#### Developer Tools
- [x] **CLI Enhancements** — Zone switching, event triggers
- [x] **Debug Mode** — Real-time flow inspection
- [x] **Webhook Replay** — Re-trigger past events

---

## Upcoming Phase

### Production Excellence (v6.0) 🛡️

Focus on long-term sustainability, reliability, and automated delivery.

#### Standardized CI/CD
- [ ] **Unified Pipelines** — GitHub Actions for all repositories
- [ ] **Automated Release** — Semantic versioning for SDKs and CLI
- [ ] **Infrastructure as Code** — Managed Kubernetes/Docker configs

#### Quality Assurance
- [ ] **Full Test Coverage** — Unit tests for all UI components and services
- [ ] **E2E Integration Suite** — Cross-repo validation tests
- [ ] **Load & Stress Testing** — Benchmarking the flow engine

#### Operational Excellence
- [ ] **Advanced Logging** — Structured JSON logging across all services
- [ ] **Performance Tuning** — Optimizing Kafka and DB bottlenecks
- [ ] **Documentation v2** — Comprehensive "How-to" guides for developers

---

## Monetization Strategy

| Tier | Zones | Events/mo | Price |
|------|-------|-----------|-------|
| **Free** | 1 | 1,000 | $0 |
| **Starter** | 3 | 10,000 | $29/mo |
| **Pro** | Unlimited | 100,000 | $99/mo |
| **Enterprise** | Custom | Custom | Contact |

Revenue drivers:
- Zone count
- Event volume
- Notification credits (WhatsApp, SMS)
- Third-party plugins
- Hosted execution
- SLA guarantees

---

## Repository Structure

| Repo | Responsibility |
|------|----------------|
| `fintech-ecosystem` | Core engine (auth, zones, events, flows) |
| `fintech-sdk-node` | Node.js SDK |
| `fintech-sdk-go` | Go SDK |
| `fintech-sdk-python` | Python SDK |
| `fintech-ui` | React components |
| `fintech-automation` | Flow Builder UI |
| `sapliy-cli` | Developer CLI |
| `fintech-docs` | Documentation site |

See [ARCHITECTURE.md](../ARCHITECTURE.md) for the full system design.

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## License

MIT © [Sapliy](https://github.com/sapliy)
