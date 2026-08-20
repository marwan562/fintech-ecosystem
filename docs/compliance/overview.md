# Enterprise Compliance Overview

This section documents the compliance program for the Sapliy Ecosystem Managed Cloud. We adhere to strict security and data protection standards to ensure the safety of financial data.

## Compliance Scope

| Standard | Level | Description | Status |
|----------|-------|-------------|--------|
| **SOC 2** | Type I | Security, Availability, Confidentiality, Processing Integrity, Privacy. | 🚧 In Progress |
| **PCI-DSS** | Level 3/4 | SAQ A-EP (Service Provider). For merchants/platforms outsourcing payment redundancy. | 🚧 In Progress |

## Shared Responsibility Model

Security is a shared responsibility between the Sapliy Cloud (Us) and the Customer (You).

### 1. Our Responsibility (Security OF the Cloud)
- **Physical Security**: Managed by AWS (Data centers, power, cooling).
- **Network Security**: VPC configuration, firewalls (Security Groups), DDoS protection (AWS Shield).
- **Infrastructure**: Patching OS, Kubernetes, Databases, and core software.
- **Application Logic**: Secure coding of the Sapliy primitives (Ledger, Payments).
- **Access Control**: Managing administrative access to the platform.

### 2. Customer Responsibility (Security IN the Cloud)
- **Access Management**: Managing your own API keys and Dashboard users (strong passwords, MFA).
- **Data Classification**: Deciding what data effectively enters the "Description" or "Metadata" fields.
- **Webhooks**: Verifying webhook signatures to prevent spoofing.
- **Compliance**: Your own PCI-DSS compliance if you handle raw card data on your frontend (we minimize this via tokenization).

## Compliance Roadmap

### Phase 1: Readiness (Current)
- Implementation of technical controls (Encryption, Logging, RBAC).
- Policy creation (Incident Response, Access Policy).
- Internal Audit/Gap Analysis.

### Phase 2: Audit
- Engagement with a CPA firm (for SOC2).
- Engagement with a QSA (for PCI-DSS, if Level 1 required later).
- Evidence collection period (3-6 months for Type II).

### Phase 3: Certification
- Final Report issuance.
- Annual renewal and penetration testing.
