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

### Developer Experience (v3.x) 🔄
- [ ] **Complete SDK Coverage** — All APIs in all SDKs
- [ ] **Comprehensive Examples** — Real-world use cases
- [ ] **SDK Publishing** — npm, PyPI, Go modules
- [ ] **OpenAPI-based Generation** — Auto-generate SDKs
- [ ] **Advanced Caching** — Redis for hot paths
- [ ] **Batch Operations** — Bulk APIs

### Ecosystem Packages (v4.x)
- [ ] **@sapliyio/fintech-ui** — React components
- [ ] **fintech-testing** — Test utilities
- [ ] **fintech-cli v2** — Enhanced CLI
- [ ] **fintech-examples** — Sample apps
- [ ] **Documentation Site** — VitePress docs

---

## Upcoming Phase

### Zone & Automation Platform (v5.x) 🚀

The next major evolution — transforming from a payment processor into a full automation platform.

#### Core Zone Features
- [ ] **Zone Management API** — CRUD for zones
- [ ] **Test/Live Mode Isolation** — Separate keys, logs, flows
- [ ] **Zone-Scoped Events** — Events bound to zones
- [ ] **Zone Templates** — Quick-start configurations

#### Flow Engine
- [ ] **Visual Flow Builder** — Drag-and-drop UI
- [ ] **Event Triggers** — SDK, webhooks, schedule
- [ ] **Logic Nodes** — Conditions, filters, approvals
- [ ] **Action Nodes** — Webhooks, notifications, audit

#### Policy Engine
- [ ] **Phase 1**: Hardcoded policies (admin, finance roles)
- [ ] **Phase 2**: JSON policy language
- [ ] **Phase 3**: Full OPA-style engine

#### Developer Tools
- [ ] **CLI Enhancements** — Zone switching, event triggers
- [ ] **Debug Mode** — Real-time flow inspection
- [ ] **Webhook Replay** — Re-trigger past events

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
