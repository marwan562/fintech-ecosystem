# 🗺️ Product Roadmap

This document outlines the strategic plan to evolve this microservices ecosystem into a production-grade, open-source fintech platform—a true self-hosted alternative to Stripe.

## 🌟 Vision
To provide a developer-first, scalable, and open-source financial infrastructure that any company can run on their own cloud.

---

## Phase 1: Open Source Foundation (Q1)
*Focus: Community, Documentation, and Developer Experience.*

- [x] **Community Standards**: Add `CONTRIBUTING.md`, Code of Conduct, and Pull Request templates.
- [x] **CI/CD Pipelines**: Implement GitHub Actions for:
    - [x] Automated Linting (`golangci-lint`)
    - [x] Unit & Integration Tests
    - [x] Docker Image Building
- [x] **Security Hardening**:
    - [x] Dependency scanning (Dependabot)
    - [x] Secret scanning in CI
    - [x] API Key hashing improvements

## Phase 2: Hyper-Scale Infrastructure (Q2)
*Focus: Reliability, Observability, and Performance.*

- [x] **Kubernetes Support**:
    - [x] K8s manifests for all microservices (`deploy/k8s`).
    - [x] Helm Charts for "one-click" deployment.
- [x] **Observability Stack**:
    - [x] Distributed Tracing (OpenTelemetry + Jaeger/Tempo).
    - [x] Centralized Metrics (Prometheus + Grafana Dashboards).
    - [x] Structured Logging (ELK/Loki).
- [x] **Database Engineering**:
    - [x] Automated schema migrations (`golang-migrate`).
    - [x] Connection pooling tuning.

## Phase 3: Developer Experience & Platform Readiness (Q3)
*Focus: Integration, SDKs, and "Platform" capabilities.*

- [x] **SDK Generation Pipeline**:
    - [x] Automate generation of `@fintech/node-sdk` and `fintech-python-sdk`.
    - [x] Add `google.api.http` annotations to Proto files for OpenAPI spec generation.
- [x] **Auth & Integration**:
    - [x] Upgrade Auth Service to support **OAuth2 / OIDC** (Client Credentials, Auth Code).
    - [x] Implement granular API Scopes (e.g., `payments:read`, `ledger:write`).
- [x] **Webhook Reliability**:
    - [x] Retry policies with exponential backoff.
    - [x] Webhook signing for security (HMAC).

## Phase 4: Ecosystem & Monetization (Q4)
*Focus: Enabling a marketplace business model and Enterprise features.*

- [ ] **"Connect" Marketplace Architecture**:
    - [ ] **Connect Service**: Manage "Connected Accounts" (sub-merchants).
    - [ ] **Revenue Splitting**: Logic to split transaction fees between Platform, Developer, and Merchant.
- [ ] **Enterprise Features**:
    - [ ] **SSO**: SAML/OIDC integration.
    - [ ] **Audit Logs**: Enhanced visualization for compliance.
    - [ ] **Team Management**: RBAC for organization teams.

## Phase 5: Future & Community
*Focus: Sustainable Open Source Growth.*

- [ ] **Plugin System**: Manifest-based extensions for the Dashboard.
- [ ] **Fraud Detection**: Basic rule-based engine.
- [ ] **Multi-Currency Support**.

---

## 🤝 Contributing
We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) to get started.
