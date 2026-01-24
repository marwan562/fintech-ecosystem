# 🗺️ Product Roadmap

This document outlines the strategic plan to evolve this microservices ecosystem into a production-grade, open-source fintech platform—a true self-hosted alternative to Stripe.

## 🌟 Vision
To provide a developer-first, scalable, and open-source financial infrastructure that any company can run on their own cloud.

---

## Phase 1: Open Source Foundation (Q1)
*Focus: Community, Documentation, and Developer Experience.*

- [ ] **Community Standards**: Add `CONTRIBUTING.md`, Code of Conduct, and Pull Request templates.
- [ ] **CI/CD Pipelines**: Implement GitHub Actions for:
    - Automated Linting (`golangci-lint`)
    - Unit & Integration Tests
    - Docker Image Building
- [ ] **Security Hardening**:
    - Dependency scanning (Dependabot)
    - Secret scanning in CI
    - API Key hashing improvements

## Phase 2: Hyper-Scale Infrastructure (Q2)
*Focus: Reliability, Observability, and Performance.*

- [x] **Kubernetes Support**:
    - [x] K8s manifests for all microservices (`deploy/k8s`).
    - [ ] Helm Charts for "one-click" deployment.
- [ ] **Observability Stack**:
    - [x] Distributed Tracing (OpenTelemetry + Jaeger/Tempo).
    - [ ] Centralized Metrics (Prometheus + Grafana Dashboards).
    - [ ] Structured Logging (ELK/Loki).
- [ ] **Database Engineering**:
    - Automated schema migrations (`golang-migrate`).
    - Connection pooling tuning.

## Phase 3: Stripe Equivalence (Q3)
*Focus: Feature Parity and Business Logic.*

- [ ] **Admin Dashboard**:
    - A Next.js/React web application for managing the ecosystem.
    - View transactions, manage customers, and inspect webhooks.
- [ ] **Subscription Engine**:
    - New microservice for recurring billing.
    - Support for plans, billing cycles, and pro-ration.
- [ ] **Developer SDKs**:
    - Official Node.js and Python clients.
    - Typed `go` client for internal service communication.
- [ ] **Webhook Reliability**:
    - Retry policies with exponential backoff.
    - Webhook signing for security.

## Phase 4: Advanced Fintech (Q4)
*Focus: Compliance and Advanced Features.*

- [ ] **Fraud Detection**: Basic rule-based engine for flagging suspicious transactions.
- [ ] **Multi-Currency**: Support for multiple currencies and FX handling.
- [ ] **Compliance**: Tools to help with PCI-DSS readiness.

---

## 🤝 Contributing
We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) to get started.
