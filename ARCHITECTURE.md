# Sapliy Fintech Architecture

**Event-driven automation & policy platform for fintech and business flows**

> Hybrid SaaS + Self-Hosted: One codebase for small/medium companies (SaaS) and large enterprises (self-hosted). Open-source first, enterprise-ready.

---

## Key Principles

Sapliy is built on five core principles that guide all architectural decisions:

1. **One Core Codebase** → Works seamlessly for both SaaS and self-hosted deployments
2. **Hybrid-First Design** → Small users = SaaS (easy), Big users = self-hosted (control)
3. **Open-Source Foundation** → Build trust, get community feedback, encourage contributions
4. **Safety-First Automation** → Test/live zones reduce risk, audit trails for compliance
5. **Extensibility Everywhere** → SDKs, UI, policies, connectors — all pluggable

---

## Core Mental Model

Everything in Sapliy reduces to **4 core concepts**:

```mermaid
graph TD
    subgraph Organization["🔑 Organization"]
        direction TB
        subgraph ZoneProd["🧩 Zone: Production"]
            direction TB
            P_Events[⚡ Events]
            P_Flows[🔄 Flows]
            P_Events --> P_Flows
        end

        subgraph ZoneStaging["🧩 Zone: Staging"]
            direction TB
            S_Events[🧪 Events]
            S_Flows[🔄 Flows]
            S_Events --> S_Flows
        end
    end

    subgraph FlowEngine["⚙️ Execution Engine"]
        Logic{Logic & Rules}
        Approval[👤 Approval]
        Audit[📝 Audit]
    end

    subgraph Outputs["📤 Actions & Ledger"]
        Ledger[(💰 Ledger)]
        Webhook[🌐 Webhook]
        Notify[✉️ Notify]
    end

    P_Flows --> Logic
    S_Flows --> Logic

    Logic --> Approval
    Approval --> Audit
    Audit --> Webhook
    Audit --> Notify
    Audit --> Ledger

    classDef zone fill:#f9f9f9,stroke:#333,stroke-width:1px;
    class ZoneProd,ZoneStaging zone;
```

### 🔑 Organization

- Owns everything
- Has users, teams, policies
- Root of all access control

### 🧩 Zone

**The key abstraction.** A zone is an isolated automation space combining:

- `sapliy_secret_key` (server-side)
- `sapliy_publishable_key` (client-side)
- **Mode**: `test` | `live`
- Flows, Events, Logs

Think: _Stripe Account + Webhook Endpoint combined_

### ⚡ Event

Everything is an event:

- From SDK (`sapliy.emit("checkout.clicked", {...})`)
- From UI interactions
- From external providers (Stripe, payment gateways)
- From the gateway itself

**No event → nothing happens.**

### 🔄 Flow (Automation)

Flows listen to:

- Event type
- Zone

Then execute:

- Logic (conditions, filters, approvals)
- Actions (webhooks, notifications, audit logs)
- Policy checks

**This is the core value proposition.**

---

## Deployment Models

Sapliy runs on the **same codebase** in two modes:

### 🌐 SaaS (Hosted by Sapliy)

- **Target**: Startups, SMBs, fast-moving teams
- **Endpoint**: `https://api.sapliy.io`
- **Setup**: Sign up → get keys → use SDKs
- **Security**: SOC 2, encrypted at rest, DDoS protection
- **Cost**: Pay-per-usage + subscriptions
- **Maintenance**: Zero — we handle everything

### 🏢 Self-Hosted (Enterprise)

- **Target**: Enterprises, regulated industries, data sovereignty requirements
- **Deployment**: Docker, Kubernetes, Helm charts
- **Endpoint**: `https://fintech.yourcompany.com` (your domain)
- **Security**: Your VPC/infrastructure, audit controls, compliance-ready
- **Cost**: License fee + support
- **Maintenance**: Your team controls upgrades, patches

**Both use identical backends.** The difference is _where_ it runs.

---

## Repository Responsibility Matrix

| Repository             | Purpose                                                                  | Talks To                           | Does NOT                                         |
| ---------------------- | ------------------------------------------------------------------------ | ---------------------------------- | ------------------------------------------------ |
| **sapliy-core**  | The brain: auth, orgs, zones, keys, events, flows, ledger, policy engine | PostgreSQL, Kafka, Redis, RabbitMQ | Care about UI, SDK language                      |
| **fintech-sdk-node**   | Node.js SDK: emit events, verify signatures, retry, test/live mode       | sapliy-core API              | Contain business logic or flow execution         |
| **fintech-sdk-go**     | Go SDK: same as Node                                                     | sapliy-core API              | Contain business logic or flow execution         |
| **fintech-sdk-python** | Python SDK: same as Node                                                 | sapliy-core API              | Contain business logic or flow execution         |
| **sapliy-ui**         | React components: checkout, payment forms, dashboards                    | Uses publishable_key only          | Execute logic, hold secrets, admin actions       |
| **sapliy-console** | Flow Builder UI: visual automation editor                                | sapliy-core only             | Execute logic, talk directly to SDK users        |
| **fintech-testing**    | Testing toolkit: local flow validation, CI/CD integration                | sapliy-core API              | Contain business logic, execute production flows |
| **sapliy-cli**         | Developer CLI: login, listen, trigger, debug                             | sapliy-core WebSocket        | Hold state, execute production logic             |
| **sapliy-docs**       | Documentation site                                                       | Static content                     | —                                                |

---

## System Architecture

```mermaid
graph TB
    subgraph "Developer Experience"
        SDK[SDK: Node/Go/Python]
        CLI[Sapliy CLI]
        UI[Fintech UI Components]
    end

    subgraph "Automation Layer"
        FlowBuilder[Flow Builder UI]
    end

    subgraph "Core Platform - sapliy-core"
        Gateway[API Gateway :8080]
        Auth[Auth Service :8081]
        Payments[Payments :8082]
        Ledger[Ledger :8083]
        Zones[Zone Manager]
        FlowEngine[Flow Engine]
        Events[Event Bus]
    end

    subgraph "Infrastructure"
        PG[(PostgreSQL)]
        Kafka[(Kafka/Redpanda)]
        Redis[(Redis)]
        RMQ[(RabbitMQ)]
    end

    SDK --> Gateway
    CLI --> Gateway
    UI --> Gateway
    FlowBuilder --> Gateway

    Gateway --> Auth
    Gateway --> Zones
    Gateway --> Payments
    Gateway --> Ledger

    Payments --> Events
    Events --> FlowEngine
    FlowEngine --> Kafka
    FlowEngine --> RMQ

    Auth --> PG
    Payments --> PG
    Ledger --> PG
    Zones --> PG
```

---

## Zone & Key Model

Every zone operates in two modes with separate isolation:

|                     | Test Mode                 | Live Mode                 |
| ------------------- | ------------------------- | ------------------------- |
| **Secret Key**      | `sk_test_...`             | `sk_live_...`             |
| **Publishable Key** | `pk_test_...`             | `pk_live_...`             |
| **Logs**            | Separate                  | Separate                  |
| **Flows**           | Can be shared or separate | Can be shared or separate |
| **Events**          | Test events only          | Production events         |

**This reduces fear for companies to experiment without risking production.**

---

## Security Model

### 🔐 Webhook Security

All webhooks include:

- **Signature header**: HMAC-SHA256 with rotating keys
- **Event ID**: Idempotency & deduplication
- **Zone ID**: Context isolation
- **Timestamp**: Replay protection (±5 min window)
- **TLS 1.3**: Encrypted in-flight

```http
POST /webhook HTTP/1.1
X-Sapliy-Signature: sha256=abc123...
X-Sapliy-Event-ID: evt_123
X-Sapliy-Event-Type: payment.completed
X-Sapliy-Timestamp: 1706972400
X-Sapliy-Signature-Version: 1
```

### 🔑 API Key Management

Keys can be scoped to specific operations & resources:

- `events:emit` — Send events
- `flows:read` — View flows
- `flows:write` — Create/edit flows
- `flows:execute` — Trigger flows
- `zones:read` — List zones
- `admin:*` — Full access (restricted to admins)
- **Zone-scoped keys**: Access limited to specific zones

Each key includes:

- Creation & expiration timestamps
- Last rotation date
- Audit logs of all uses

### 🛡️ Enterprise Security Features (Self-Hosted)

#### Multi-Layer Access Control

- **RBAC**: Role-based access (Admin, Developer, Auditor, Read-only)
- **ABAC**: Attribute-based access (team, department, region)
- **Custom PBAC**: Policy-based access with OPA integration
- **IP Whitelisting**: Restrict API access by IP ranges
- **VPC Peering**: Direct network connectivity for self-hosted

#### Data Protection

- **Encryption at Rest**: AES-256 for all data, customer-managed keys (CMK)
- **Encryption in Transit**: TLS 1.3 enforced, mTLS support
- **Field-Level Encryption**: Sensitive data (PII, tokens) encrypted separately
- **Data Masking**: Automatically mask sensitive fields in logs
- **Key Rotation**: Automatic rotation policies (90 days default)

#### Compliance & Audit

- **Audit Logs**: Immutable, tamper-proof event logs with digital signatures
- **SOC 2 Type II**: SaaS deployment certified
- **GDPR/CCPA**: Data subject requests, data deletion, consent tracking
- **PCI-DSS**: Payment-safe by design (no direct card storage)
- **HIPAA Ready**: Encryption, audit trails, BAA support (self-hosted)
- **Compliance Reports**: Automated compliance exports (HIPAA, SOC 2, ISO 27001)

#### Threat Detection & Prevention

- **DDoS Protection**: Rate limiting, WAF rules (SaaS + optional for self-hosted)
- **Intrusion Detection**: Anomaly detection for unusual API patterns
- **Secret Scanning**: Monitor logs for exposed credentials
- **Vulnerability Scanning**: Automated CVE detection & patching
- **Incident Response**: Automated alerting, forensic logging

#### Network Security (Self-Hosted)

- **Private VPC Deployment**: Isolated network infrastructure
- **NAT Gateway Support**: Egress traffic filtering
- **Network Policies**: Kubernetes NetworkPolicy enforcement
- **Secrets Management**: HashiCorp Vault / AWS Secrets Manager integration

---

## Development & Release Plan

### Phase 1: MVP / SaaS Launch (Months 1-3)

**Goal**: Get SaaS version live with core features

- ✅ Core backend (sapliy-core)
- ✅ Event ingestion & flow execution
- ✅ Auth, zones, keys, ledger
- ✅ Test/Live mode support
- ✅ Node.js SDK
- ✅ Basic UI components
- ✅ CLI for testing flows locally
- 📊 Monitoring & alerting
- 🔐 Basic SOC 2 compliance

### Phase 2: Open-Source & SDK Expansion (Months 4-6)

**Goal**: Build community, expand language support

- 📦 Publish sapliy-core as open-source (MIT license)
- 🐍 Python SDK (`sapliyio-fintech`)
- 🔵 Go SDK (`fintech-sdk-go`)
- 🧪 Testing toolkit (`@sapliyio/fintech-testing`)
- 📚 Complete documentation
- 🎓 Example applications (checkout flow, payment routing, notifications)
- 🤝 Open issues for community contributions

### Phase 3: Self-Hosted Option (Months 7-9)

**Goal**: Enable enterprises to self-host with full compliance support

- 🐳 Docker images for all services
- ☸️ Kubernetes manifests & Helm charts
- 📖 Deployment guides (AWS, GCP, Azure, on-prem)
- 🔐 Enterprise security features (see below)
- 📋 HIPAA/PCI-DSS compliance documentation
- 🔄 Migration tools from SaaS to self-hosted
- 💼 Enterprise support tier
- 🎯 Configurable endpoints for SDKs

### Phase 4: Monetization & Advanced Features (Months 10-12)

**Goal**: Build sustainable business model, add enterprise features

- 💰 Usage-based pricing (events, zones, API calls)
- 🔌 Premium plugins (SMS, WhatsApp, 3rd-party integrations)
- 👥 Advanced team & role management
- 📊 Advanced analytics & reporting
- 🔐 Enhanced compliance reports
- 🎯 Priority support & SLA guarantees
- 🚀 Multi-region deployment for self-hosted
- 🤖 AI-powered flow suggestions

---

## Enterprise-Grade Features for Self-Hosted

### High Availability & Disaster Recovery

- **Multi-region replication**: Automatic failover across regions
- **Zero-downtime deployments**: Blue-green deployments built-in
- **Backup & restore**: Automated daily backups with point-in-time recovery
- **RTO/RPO guarantees**: <15 min RTO, <1 min RPO (configurable)
- **Database failover**: Primary-replica setup with automatic promotion
- **Load balancing**: Multi-zone load balancing for production

### Observability & Monitoring

- **Distributed tracing**: Full request tracing with OpenTelemetry
- **Metrics**: Prometheus-compatible metrics on all services
- **Logging**: ELK stack integration, structured JSON logs
- **Alerting**: PagerDuty, Slack, email integrations
- **Performance profiling**: CPU, memory, goroutine profiling built-in
- **Health checks**: Liveness & readiness probes for Kubernetes

### Advanced Policy Engine

- **OPA (Open Policy Agent)**: Embed REGO policies for complex rules
- **Attribute-based access control (ABAC)**: Fine-grained permission rules
- **Dynamic policy reloading**: Update policies without restart
- **Policy versioning**: Track and audit all policy changes
- **Conditional execution**: Rules based on context, time, geography

### Integration & Extensibility

- **Webhook retries**: Exponential backoff with configurable limits
- **Custom headers**: Add custom headers to all outbound webhooks
- **Event transformation**: Transform events before passing to flows
- **Flow plugins**: Custom action types (integrations, validators)
- **Third-party connectors**: Pre-built connectors (Stripe, Slack, PagerDuty, etc.)

### Data Management at Scale

- **Event archival**: Move old events to cold storage (S3, GCS)
- **Data retention policies**: Automatic purge based on retention rules
- **Bulk operations**: Export/import flows, zones, configurations
- **Database optimization**: Automatic index tuning and query optimization
- **Partitioning**: Event and log partitioning by time/zone for performance

---

## Policy-Based Access Control (PBAC)

### Phase 1: Hardcoded Policies (Current)

```
- "Who can create zones" → Admin only
- "Who can deploy live flows" → Admin + Finance
```

### Phase 2: Simple JSON Policies

```json
{
  "allow": ["flow.deploy"],
  "when": { "role": "admin" }
}
```

### Phase 3: Full Policy Engine (Future)

OPA-style policy language for complex rules.

---

## Monetization Strategy

| Tier           | Features                                               | Price         |
| -------------- | ------------------------------------------------------ | ------------- |
| **Free**       | 1 zone, 1K events/month, test mode only                | $0            |
| **Starter**    | 3 zones, 10K events/month, live mode                   | $29/mo        |
| **Pro**        | Unlimited zones, 100K events/month, advanced analytics | $99/mo        |
| **Enterprise** | Custom pricing, SLA, compliance, priority support      | Contact sales |

### Revenue Drivers

- **Zone count**: Additional zones beyond tier limits
- **Event volume**: Overage charges per 1M events
- **Notifications**: SMS, WhatsApp, email beyond included quota
- **Third-party plugins**: Premium integrations (Stripe, Slack, etc.)
- **Hosted execution**: Serverless flow execution
- **SLA guarantees**: 99.99% uptime, <100ms latency
- **Advanced features**: Multi-region, custom policies, white-label

### Self-Hosted Licensing

- **Startup License**: $1,999/year (up to 50 employees)
- **Growth License**: $9,999/year (up to 500 employees)
- **Enterprise License**: Custom pricing (unlimited)
- **Support tiers**: Community, Standard, Premium, Enterprise

---

## Integration Capabilities

### Out-of-the-Box Connectors

- **Payment Gateways**: Stripe, Square, PayPal, Razorpay
- **Communication**: Slack, Discord, Telegram, SendGrid, Twilio
- **Monitoring**: PagerDuty, DataDog, New Relic, Sentry
- **Databases**: PostgreSQL, MySQL, MongoDB, DynamoDB
- **Cloud Storage**: S3, GCS, Azure Blob
- **Authentication**: Auth0, Okta, Azure AD

### Custom Connectors

- **Webhook transform**: Modify event shape before processing
- **Custom validators**: Validate events against business rules
- **External APIs**: Call any REST API with retry logic
- **Database queries**: Execute SQL and use results in flows

---

## SDK & Client Specifications

### SDK Features (All Languages)

```javascript
// Node.js example
const sapliy = new Sapliy({
  secretKey: "sk_live_...",
  endpoint: "https://api.sapliy.io", // or self-hosted
});

// Send event
await sapliy.emit("payment.completed", {
  orderId: "123",
  amount: 100.5,
  currency: "USD",
});

// Verify webhook signature
const isValid = sapliy.verifyWebhook(body, headers["x-sapliy-signature"]);

// Listen for events (CLI)
await sapliy.listen("payment.*");
```

### Supported Languages

- **Node.js/JavaScript**: `@sapliyio/fintech`
- **Python**: `sapliyio-fintech`
- **Go**: `fintech-sdk-go`
- **Future**: Ruby, Java, .NET, PHP

---

## Testing & CI/CD Integration

### Local Testing

```bash
# Install CLI
npm install -g @sapliyio/sapliy-cli

# Login
sapliy login

# List zones
sapliy zones list

# Trigger flow
sapliy flows trigger --zone=prod --flow=checkout_completed

# Listen to webhooks
sapliy listen --zone=test
```

### Testing Toolkit

```python
from sapliyio_fintech.testing import TestClient

client = TestClient(endpoint='http://localhost:8080')

# Emit test event
response = client.emit_event(
  zone_id='zone_test',
  event_type='payment.completed',
  data={'amount': 100}
)

# Assert flow executed
assert response.status_code == 200
assert response.flow_executions[0].status == 'success'
```

### CI/CD Examples

- **GitHub Actions**: Validate flows on pull request
- **GitLab CI**: Deploy flows on merge to main
- **Jenkins**: Run flow tests in pipeline
- **Pre-commit hooks**: Lint flows before commit

---

## Data Model & Storage

### Core Tables

```sql
-- Organizations
CREATE TABLE organizations (
  id UUID PRIMARY KEY,
  name VARCHAR NOT NULL,
  created_at TIMESTAMP,
  tier VARCHAR (free|starter|pro|enterprise)
);

-- Zones (isolated automation spaces)
CREATE TABLE zones (
  id UUID PRIMARY KEY,
  org_id UUID REFERENCES organizations,
  name VARCHAR NOT NULL,
  mode VARCHAR (test|live),
  created_at TIMESTAMP
);

-- API Keys
CREATE TABLE api_keys (
  id UUID PRIMARY KEY,
  zone_id UUID REFERENCES zones,
  secret_key VARCHAR UNIQUE,
  publishable_key VARCHAR UNIQUE,
  permissions JSONB,
  expires_at TIMESTAMP,
  last_used_at TIMESTAMP
);

-- Events
CREATE TABLE events (
  id UUID PRIMARY KEY,
  zone_id UUID REFERENCES zones,
  event_type VARCHAR,
  payload JSONB,
  created_at TIMESTAMP,
  INDEX (zone_id, created_at)
);

-- Flows
CREATE TABLE flows (
  id UUID PRIMARY KEY,
  zone_id UUID REFERENCES zones,
  name VARCHAR,
  trigger_event VARCHAR,
  definition JSONB, -- Flow definition
  enabled BOOLEAN,
  created_at TIMESTAMP
);

-- Flow Executions
CREATE TABLE flow_executions (
  id UUID PRIMARY KEY,
  flow_id UUID REFERENCES flows,
  event_id UUID REFERENCES events,
  status VARCHAR (pending|running|success|failed),
  result JSONB,
  started_at TIMESTAMP,
  completed_at TIMESTAMP
);

-- Audit Logs
CREATE TABLE audit_logs (
  id UUID PRIMARY KEY,
  org_id UUID REFERENCES organizations,
  actor_id UUID,
  action VARCHAR,
  resource_type VARCHAR,
  resource_id UUID,
  changes JSONB,
  created_at TIMESTAMP,
  INDEX (org_id, created_at)
);

-- Ledger (Financial events)
CREATE TABLE ledger_entries (
  id UUID PRIMARY KEY,
  zone_id UUID REFERENCES zones,
  debit DECIMAL,
  credit DECIMAL,
  description VARCHAR,
  reference_id UUID,
  created_at TIMESTAMP,
  INDEX (zone_id, created_at)
);
```

---

## Performance & Scalability

### Performance Targets

- **Event ingestion**: 10K+ events/sec per instance
- **Flow execution**: <100ms p99 latency
- **Webhook delivery**: 99.99% delivery rate
- **API response time**: <50ms p99 for all endpoints
- **Database queries**: <10ms p99 for common queries

### Scaling Strategy

- **Horizontal scaling**: Stateless services behind load balancer
- **Event partitioning**: Partition events by zone and time
- **Flow queue**: Kafka/RabbitMQ for async flow execution
- **Cache layer**: Redis for frequently accessed data
- **Database sharding**: Shard by organization (future)

---

## Getting Started

---

## Getting Started

### For SaaS Users

1. Sign up at `https://sapliy.io`
2. Create a zone (test or live)
3. Copy API keys
4. Install SDK: `npm install @sapliyio/fintech`
5. Emit events and build flows

### For Self-Hosted Deployment

1. Pull Docker images
2. Configure PostgreSQL, Redis, Kafka
3. Deploy with Helm chart
4. Configure TLS, secrets management
5. Point SDKs to self-hosted endpoint

---

## Operational Excellence for Enterprise

### Disaster Recovery & Business Continuity

- **RTO (Recovery Time Objective)**: <15 minutes
- **RPO (Recovery Point Objective)**: <1 minute
- **Automated backups**: Hourly incremental, daily full backups
- **Multi-region failover**: Automatic detection and failover to standby region
- **Data redundancy**: 3x replication across availability zones
- **Chaos engineering**: Regular failure simulations to validate recovery

### Monitoring & Observability (SaaS & Self-Hosted)

- **Real-time alerting**: PagerDuty, Slack, SMS for critical incidents
- **Custom dashboards**: Grafana dashboards for all metrics
- **Service-level objectives (SLOs)**:
  - Availability: 99.99% (SaaS), 99.95% (Self-hosted)
  - Latency: p99 < 100ms for all API endpoints
  - Error rate: < 0.1%
- **Incident management**: Automated runbooks, post-mortems, root cause analysis
- **Cost monitoring**: Track and optimize cloud spend (self-hosted)

### Change Management & Deployments

- **Versioning strategy**: Semantic versioning, backwards compatibility guaranteed
- **Deployment strategies**:
  - Blue-green deployments for zero-downtime updates
  - Canary deployments to detect issues early
  - Automatic rollback on detection of errors
- **Change windows**: Schedule maintenance windows, communicate in advance
- **Database migrations**: Zero-downtime schema changes with dual-write strategies

### Security Operations

- **Vulnerability management**:
  - Weekly dependency scanning via GitHub/Snyk
  - Monthly penetration testing
  - Bug bounty program for community security researchers
- **Patch management**: Critical patches within 24 hours, regular patches weekly
- **Access logging**: Track all administrative actions with immutable audit trails
- **Secret rotation**: Automatic rotation of database credentials, API keys
- **Incident response**: On-call rotation, incident commander process, 1-hour response SLA

### Compliance & Governance

- **Automated compliance**: CI/CD checks for compliance violations
- **Policy enforcement**: Terraform/Helm validation against security policies
- **Data governance**: Data classification, retention policies, export controls
- **Regular audits**: Quarterly internal audits, annual third-party SOC 2 audits
- **Regulatory reporting**: Automated reports for HIPAA, PCI-DSS, GDPR, CCPA

### Documentation & Knowledge Management

- **API documentation**: OpenAPI 3.0 specs, auto-generated SDKs
- **Runbooks**: Step-by-step guides for common operational tasks
- **Architecture decision records (ADRs)**: Document all significant decisions
- **Playbooks**: Response procedures for known issues
- **Knowledge base**: Searchable, version-controlled documentation

---

## Getting Started

1. **Run the ecosystem**: `docker-compose up -d` in `sapliy-core`
2. **Create a zone**: Use the API or Flow Builder UI
3. **Install SDK**: `npm install @sapliyio/fintech`
4. **Emit events**: `sapliy.emit("checkout.completed", { amount: 100 })`
5. **Build flows**: Use the Flow Builder to react to events

---

## Hybrid Deployment Decision Matrix

| Aspect             | SaaS                         | Self-Hosted                            |
| ------------------ | ---------------------------- | -------------------------------------- |
| **Target Users**   | Startups, SMBs, Fast-growing | Enterprises, Regulated, Data-sovereign |
| **Time to Setup**  | Minutes                      | Days/Weeks                             |
| **Maintenance**    | 0% (Managed by Sapliy)       | 30-40% (Your team)                     |
| **Compliance**     | SOC 2, DDoS, CDN             | HIPAA, FedRAMP, Custom policies        |
| **Scalability**    | Auto-scaling, unlimited      | Manual + Kubernetes orchestration      |
| **Data Residency** | Sapliy Infrastructure        | Your VPC/On-prem                       |
| **Cost Model**     | Usage-based (pay-as-you-go)  | Annual license + support               |
| **API Endpoint**   | `https://api.sapliy.io`      | `https://fintech.yourcompany.com`      |
| **SDKs Work?**     | ✅ Yes (both modes)          | ✅ Yes (configurable endpoint)         |
| **Flow Builder**   | ✅ Cloud UI                  | ✅ Self-hosted UI                      |
| **Audit Trails**   | Sapliy-managed               | Customer-controlled logs               |

---

## Key Decisions for Enterprise Customers

When choosing between SaaS and Self-Hosted:

### Choose **SaaS** if:

- You want to get started in minutes
- You don't have strict data residency requirements
- Your industry doesn't require HIPAA/FedRAMP
- You prefer managed infrastructure
- You're cost-sensitive and have low/medium event volumes

### Choose **Self-Hosted** if:

- You need data sovereignty (data must stay in your infrastructure)
- You're in a regulated industry (HIPAA, PCI-DSS, FedRAMP)
- You have high security/compliance requirements
- You want full control over deployment and updates
- You have existing Kubernetes/Docker infrastructure

### Hybrid Strategy:

- **Development**: Use SaaS (test mode)
- **Production**: Deploy self-hosted (live mode)
- **Backup**: Replicate critical flows to SaaS (disaster recovery)

---

## Enterprise Implementation Best Practices

### Pre-Deployment Checklist

- [ ] Infrastructure planning (VPC, subnet isolation, NAT gateways)
- [ ] Security audit of deployment environment
- [ ] SSL/TLS certificate management (self-signed or CA-signed)
- [ ] Database password management & secrets vault setup
- [ ] Backup & restore testing
- [ ] Load balancer configuration & health checks
- [ ] Monitoring & alerting infrastructure ready
- [ ] Documentation reviewed with ops/security teams
- [ ] Disaster recovery plan documented & tested
- [ ] Change management process defined

### Deployment Phases

1. **Infrastructure Setup** (Week 1-2)
   - Provision cloud resources or on-prem infrastructure
   - Set up networking, VPC peering, security groups
   - Configure TLS certificates & secrets management
   - Deploy database, message queues, caching layer

2. **Service Deployment** (Week 2-3)
   - Deploy core backend services (containers)
   - Deploy Flow Engine and Event Bus
   - Test service health checks & logging
   - Set up monitoring & alerting

3. **Integration Testing** (Week 3-4)
   - Test event ingestion with sample payloads
   - Validate flow execution & webhook delivery
   - Test failover & disaster recovery scenarios
   - Load testing (10K+ events/sec)

4. **Security Validation** (Week 4)
   - Penetration testing
   - Compliance audit (HIPAA/PCI-DSS if applicable)
   - Secret scanning & vulnerability assessment
   - Access control verification

5. **Go-Live** (Week 5)
   - Shadow production traffic
   - Gradual traffic migration
   - 24/7 monitoring & on-call support
   - Post-launch validation

### Migration from SaaS to Self-Hosted

- **Export flows**: Use bulk export API to extract all flows
- **Data migration**: Migrate events & execution logs to self-hosted
- **API key transfer**: Re-issue keys for self-hosted endpoints
- **Webhook re-registration**: Update webhook URLs if changed
- **Testing phase**: Run both systems in parallel for 1-2 weeks
- **Cutover**: Switch traffic after validation complete

### Post-Deployment Operations

- **Monthly security patches**: Apply updates to all components
- **Quarterly audits**: Review logs, access controls, policy changes
- **Annual capacity planning**: Monitor growth, scale infrastructure
- **Performance tuning**: Optimize database queries, cache hit rates
- **Incident response drills**: Test alerting and runbooks monthly

---

## Related Repositories

- [sapliy-core](https://github.com/sapliy/sapliy-core) — Core backend services (Go)
- [fintech-sdk-node](https://github.com/sapliy/fintech-sdk-node) — Node.js SDK (`@sapliyio/fintech`)
- [fintech-sdk-go](https://github.com/sapliy/fintech-sdk-go) — Go SDK (`fintech-sdk-go`)
- [fintech-sdk-python](https://github.com/sapliy/fintech-sdk-python) — Python SDK (`sapliyio-fintech`)
- [sapliy-ui](https://github.com/sapliy/sapliy-ui) — React components (`@sapliyio/sapliy-ui`)
- [fintech-testing](https://github.com/sapliy/fintech-testing) — Testing toolkit (`@sapliyio/fintech-testing`)
- [sapliy-console](https://github.com/sapliy/sapliy-console) — Flow Builder UI (React)
- [sapliy-cli](https://github.com/sapliy/sapliy-cli) — Developer CLI (`@sapliyio/sapliy-cli`)
- [sapliy-docs](https://github.com/sapliy/sapliy-docs) — Documentation site

---

## License

MIT © [Sapliy](https://github.com/sapliy)

---

## Support & Community

- **Discord**: [Sapliy Community](https://discord.gg/sapliy)
- **GitHub Issues**: [Report bugs](https://github.com/sapliy/sapliy-core/issues)
- **Enterprise Support**: [contact@sapliy.io](mailto:contact@sapliy.io)
- **Security**: [security@sapliy.io](mailto:security@sapliy.io)

---

## Implementation Roadmap: From MVP to Enterprise Scale

### Sprint 1-4: Core Platform MVP

**Deliverables**: Minimum Viable Product for SaaS launch

- Backend API with authentication & authorization
- Zone management with test/live mode separation
- Event ingestion pipeline (10K+ events/sec)
- Flow engine with basic conditions and webhooks
- PostgreSQL data model for all core entities
- API key generation and scoping
- Basic dashboard UI (single organization)
- CLI for testing flows locally
- Docker Compose setup for local development
- API documentation (OpenAPI 3.0)

### Sprint 5-8: SaaS & SDKs

**Deliverables**: Multi-language SDK support & production readiness

- Node.js SDK with full features
- SDK signature verification for webhooks
- Rate limiting & quota management
- Monitoring & alerting setup
- Basic SOC 2 Type II compliance
- SaaS API endpoint (api.sapliy.io)
- Pricing tier implementation
- Basic analytics dashboard
- Email notifications for important events

### Sprint 9-12: Self-Hosted & Compliance

**Deliverables**: Enterprise-ready self-hosted option

- Docker images for all services
- Kubernetes manifests & Helm charts
- AWS/GCP/Azure deployment guides
- HIPAA-ready audit logging
- PCI-DSS compliance documentation
- GDPR/CCPA data handling features
- Python SDK (`sapliyio-fintech`)
- Go SDK (`fintech-sdk-go`)
- Multi-region support for self-hosted
- Enterprise license model

### Sprint 13-16: Advanced Features

**Deliverables**: Premium features & integrations

- Flow Builder visual editor (React)
- OPA policy engine integration
- Third-party connectors (Stripe, Slack, PagerDuty, etc.)
- Advanced analytics & reporting
- Custom action types & transformations
- Webhook retry with exponential backoff
- Event transformation & filtering
- Team management & RBAC
- Audit log export capabilities
- Performance optimization & caching

### Sprint 17+: Scale & Innovation

**Deliverables**: Continuous improvement & expansion

- Multi-region self-hosted deployment
- Database sharding by organization
- Flow versioning & rollback
- AI-powered flow suggestions
- Mobile app for flow monitoring
- Advanced threat detection
- Custom plugins marketplace
- White-label option for partners
- Beta: Scheduled flows & cron triggers
- Beta: Flow templates & marketplace

---

## Security Hardening Checklist for Self-Hosted

### Network Security

- [ ] Enable VPC Flow Logs for audit
- [ ] Configure security groups with least-privilege rules
- [ ] Enable WAF (Web Application Firewall) rules
- [ ] Set up DDoS protection (AWS Shield, GCP Cloud Armor)
- [ ] Configure NAT gateways for outbound traffic
- [ ] Enable VPC peering for internal communication
- [ ] Use private subnets for databases & message queues
- [ ] Implement network segmentation by function

### Access Control

- [ ] Configure IAM roles with least-privilege
- [ ] Enable MFA for all administrative access
- [ ] Set up temporary credentials (STS) for services
- [ ] Implement session recording for privileged access
- [ ] Configure bastion hosts for SSH access
- [ ] Use service accounts with minimal scopes
- [ ] Enable audit logging for all IAM changes
- [ ] Implement IP whitelisting for admin endpoints

### Data Protection

- [ ] Enable encryption at rest (AES-256) for all storage
- [ ] Configure customer-managed keys (CMK)
- [ ] Enable encryption in transit (TLS 1.3)
- [ ] Configure automatic key rotation
- [ ] Enable field-level encryption for sensitive data
- [ ] Configure database transparent encryption
- [ ] Enable Redis encryption
- [ ] Set up secure backup vaults with encryption

### Secrets Management

- [ ] Deploy HashiCorp Vault or AWS Secrets Manager
- [ ] Rotate database credentials automatically (30-90 days)
- [ ] Rotate API keys automatically (90 days)
- [ ] Implement secret scanning in CI/CD pipeline
- [ ] Audit all secret access
- [ ] Set up alerts for unauthorized access
- [ ] Configure least-privilege secret access
- [ ] Use sealed secrets for Kubernetes

### Compliance & Audit

- [ ] Enable CloudTrail / equivalent audit logging
- [ ] Configure immutable logs storage
- [ ] Set up log retention policies
- [ ] Implement log encryption
- [ ] Enable data loss prevention (DLP) tools
- [ ] Configure automated compliance scanning
- [ ] Set up vulnerability scanning
- [ ] Document data flows & classifications
- [ ] Implement data residency controls
- [ ] Set up SIEM (Security Information & Event Management)

### Incident Response

- [ ] Document incident response procedures
- [ ] Set up on-call rotation
- [ ] Configure automated alerting
- [ ] Establish SLA for incident response
- [ ] Plan for forensic logging
- [ ] Set up automated backups for recovery
- [ ] Test disaster recovery monthly
- [ ] Document post-incident procedures

---

## Performance Tuning Guide

### Database Optimization

```sql
-- Create essential indexes
CREATE INDEX idx_events_zone_created ON events(zone_id, created_at DESC);
CREATE INDEX idx_flows_zone_enabled ON flows(zone_id, enabled);
CREATE INDEX idx_flow_executions_flow_status ON flow_executions(flow_id, status);
CREATE INDEX idx_audit_logs_org_created ON audit_logs(org_id, created_at DESC);

-- Analyze query plans
EXPLAIN ANALYZE SELECT * FROM events WHERE zone_id = $1 ORDER BY created_at DESC LIMIT 100;

-- Configure connection pooling (PgBouncer)
-- max_db_connections = 100
-- pool_mode = transaction
-- min_pool_size = 10
```

### Redis Caching Strategy

```
Cache Layer:
- API keys (TTL: 5 min)
- Zone configurations (TTL: 10 min)
- Flow definitions (TTL: 30 min)
- Organization settings (TTL: 1 hour)

Cache Invalidation:
- On API key update: immediate
- On flow change: 30 second delay (eventual consistency)
- On org config change: 5 minute delay
```

### Kafka Tuning

```properties
# Producer settings
batch.size=16384
linger.ms=10
compression.type=snappy

# Consumer settings
fetch.min.bytes=1024
fetch.max.wait.ms=500
max.partition.bytes=1048576

# Topic settings
retention.ms=604800000 (7 days)
num.partitions=12 (by zone for parallelism)
replication.factor=3
```

### API Response Optimization

- Implement pagination (default: 50, max: 1000)
- Add field filtering (return only required fields)
- Use ETags for caching
- Implement gzip compression
- Add request/response caching headers
- Batch API calls where possible

---

## Monitoring & Alerting Setup

### Key Metrics to Monitor

```
System Metrics:
- CPU utilization (alert: > 80%)
- Memory usage (alert: > 85%)
- Disk space (alert: < 10% free)
- Network I/O (track trends)
- Process count (alert: > threshold)

Application Metrics:
- API request latency p99 (alert: > 100ms)
- Event ingestion rate (alert: < expected rate)
- Flow execution success rate (alert: < 99%)
- Webhook delivery success rate (alert: < 99%)
- Database query latency p99 (alert: > 10ms)

Business Metrics:
- Events processed per minute
- Flows executed per hour
- Active zones
- Active API keys
- Webhook failures per hour
```

### Alert Rules

```
Alert: High Error Rate
- Condition: error_rate > 1% for 5 minutes
- Action: Page on-call engineer

Alert: Database Replication Lag
- Condition: replication_lag_ms > 1000
- Action: Page database team

Alert: Webhook Delivery Failing
- Condition: webhook_failure_rate > 5% for 10 minutes
- Action: Create incident, notify engineering

Alert: API Key Exhaustion
- Condition: api_calls > 90% of quota
- Action: Notify organization
```

---

## Cost Optimization for Self-Hosted

### Infrastructure Cost Reduction

- Use Reserved Instances for base load (30-40% savings)
- Enable auto-scaling to match demand
- Implement spot instances for non-critical workloads
- Use object storage (S3) for log archival
- Compress old data to reduce storage costs
- Schedule non-production environments to shut down

### Operational Cost Reduction

- Implement infrastructure-as-code (Terraform)
- Automate provisioning & deprovisioning
- Use managed services where possible
- Implement chargeback models (cost allocation)
- Monitor spending with cost analysis tools
- Establish budgets and alerts
- Regular cost reviews (quarterly)

### Performance Cost Trade-offs

- Tune caching to reduce database load
- Implement request batching
- Use asynchronous processing where appropriate
- Optimize database indexes
- Archive old events (cost vs. query latency)
- Use compression for network traffic

---

## CLI-First Developer Experience: sapliy-cli

### The Unified Entry Point

The **sapliy-cli** is the professional, Node-RED-style command-line interface for developers:

```bash
# One command starts everything
sapliy dev                                    # Start backend + frontend + watch mode

# Or run separately
sapliy run                                    # Start backend services (Docker)
sapliy frontend                               # Launch Flow Builder UI (http://localhost:3000)
sapliy test --flow="checkout"                 # Test flows locally
sapliy events emit "payment.completed" {...}  # Emit test events
sapliy logs --follow                          # Stream all logs
```

### Key CLI Features

**Auto-Discovery & Setup**

- ✅ Auto-detect and use free ports (no conflicts)
- ✅ Auto-open browser when frontend starts
- ✅ Auto-create .env.local with credentials
- ✅ Auto-setup Docker Compose services
- ✅ Cross-platform (Windows, macOS, Linux)

**Development Experience**

- ✅ Unified logging from all services
- ✅ Watch mode for hot-reload
- ✅ Configuration via sapliy.json
- ✅ Webhook inspector & replay
- ✅ Event emission & listening

**Production Readiness**

- ✅ Health checks & service monitoring
- ✅ Graceful shutdown handling
- ✅ Multi-service orchestration
- ✅ Configuration management

### Real-World Developer Workflow

```bash
# Fresh start - everything with one command
$ cd my-fintech-app
$ sapliy dev

✨ Starting Sapliy in development mode...

  sapliy:run ✅ Checking Docker...
  sapliy:run ✅ Creating .env.local
  sapliy:run 🐳 Starting containers...
  sapliy:run ✅ PostgreSQL running on localhost:5432
  sapliy:run ✅ Redis running on localhost:6379
  sapliy:run ✅ Kafka running on localhost:9092
  sapliy:run ✅ API server running on http://localhost:8080

  sapliy:frontend ✅ Starting dev server...
  sapliy:frontend 🌐 Opening browser to http://localhost:3000

✨ All systems running!
   API:      http://localhost:8080
   Frontend: http://localhost:3000

Press Ctrl+C to stop
```

### Command Categories

#### Authentication

```bash
sapliy login                          # Authenticate with SaaS or self-hosted
sapliy logout                         # Remove stored credentials
sapliy whoami                         # Show current user & zone
```

#### Development Services

```bash
sapliy run                            # Start backend + event processor
sapliy frontend                       # Launch Flow Builder UI
sapliy dev                            # Combined: backend + frontend + watch
```

#### Zone Management

```bash
sapliy zones list                     # List all zones
sapliy zones create --name="my-app"   # Create new zone
sapliy zones switch --zone="prod"     # Switch active zone
sapliy zones export                   # Export zone config for backup
```

#### Flow Management

```bash
sapliy flows list                     # List flows in current zone
sapliy flows create --name="checkout" # Create flow interactively
sapliy flows deploy --flow="checkout" # Deploy to live mode
sapliy flows test --flow="checkout"   # Test flow locally
sapliy flows logs --flow="checkout"   # Stream execution logs
```

#### Event Operations

```bash
sapliy events emit "payment.completed" '{"orderId":"123"}'  # Emit test event
sapliy events listen "payment.*"      # Listen to events in real-time
sapliy events replay --after="2024-01-15" # Replay historical events
```

#### Webhook Testing

```bash
sapliy webhooks listen                # Start webhook inspector (localhost:9000)
sapliy webhooks test --url="..."      # Test webhook delivery
sapliy webhooks replay --id="evt_xxx" # Replay specific webhook
```

#### Testing & Monitoring

```bash
sapliy test --flow="checkout"         # Run tests for one flow
sapliy test --all                     # Run all flow tests
sapliy test --coverage                # Show test coverage

sapliy logs --follow                  # Stream all service logs
sapliy metrics                        # Show performance metrics
sapliy health                         # Check service health
```

### Configuration Management

#### sapliy.json (Project Configuration)

```json
{
  "name": "my-sapliy-app",
  "version": "1.0.0",
  "sapliy": {
    "apiPort": 8080,
    "uiPort": 3000,
    "services": ["postgres", "redis", "kafka"],
    "environment": {
      "LOG_LEVEL": "info",
      "TEST_MODE": true,
      "STRIPE_KEY": "sk_test_..."
    },
    "flows": ["./flows/payment.json", "./flows/notifications.json"]
  }
}
```

#### Environment Variables (Auto-Generated)

```bash
# .env.local (created by sapliy run)
API_PORT=8080
DATABASE_URL=postgresql://postgres:password@localhost:5432/sapliy
REDIS_URL=redis://localhost:6379
KAFKA_BROKERS=localhost:9092
SAPLIY_SK=sk_test_xxxxx
SAPLIY_PK=pk_test_xxxxx
```

### Impact on User Experience

**Development Time Reduction**

- Without CLI: 30 min (manual Docker, env setup, port conflicts)
- With CLI: 5 min (one command, everything auto-configured)
- **74% faster onboarding**

**Error Prevention**

- ✅ No port conflicts (auto-detect)
- ✅ No manual env setup (auto-generate)
- ✅ No service failures (health checks)
- ✅ Clear error messages

**Professional Positioning**

- Comparable to Node-RED, Docker, Kubernetes CLI
- Shows production-readiness
- Appeals to DevOps & infrastructure teams
- Encourages CLI-first workflows

### Revenue Impact

#### For SaaS

- **Faster activation**: 5 min → first event (vs 15 min)
- **Higher conversion**: Better DX → 15-20% free-to-paid
- **Community**: CLI contributions drive engagement

#### For Self-Hosted

- **Easier evaluation**: Teams can spin up in minutes
- **Professional image**: Enterprise customers appreciate CLI maturity
- **Faster deployment**: 2-3 weeks vs 4-6 weeks

#### For Monetization

- **Advanced CLI features** (enterprise add-on):
  - `sapliy audit-export` (compliance)
  - `sapliy performance-profile` (analytics)
  - `sapliy multi-region-sync` (HA)
  - Pricing: $49-$299/month
- **Professional services**: CLI-based setup & configuration
  - Pricing: $200-$350/hour

---

## SDK & Client Specifications
