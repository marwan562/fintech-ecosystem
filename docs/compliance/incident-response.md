# Incident Response Plan

## 1. Purpose
To ensure a coordinated, effective response to security incidents and system outages, minimizing impact on customers and reputation.

## 2. Severity Classification

| Level | Severity | Definition | Response Time |
|-------|----------|------------|---------------|
| **P0** | Critical | Data Breach, Total System Outage, Financial Loss. | Immediate (15m) |
| **P1** | High | Partial Outage, Performance degradation affecting <50% users. | 1 Hour |
| **P2** | Medium | Non-blocking bug, internal tool failure. | 4 Hours |
| **P3** | Low | Minor UI glitch, documentation error. | 2 Business Days |

## 3. Incident Response Team (IRT)

- **Incident Commander (IC)**: Leads the response, makes final decisions. (Usually DevOps Lead / CTO).
- **Communications Lead**: Handles internal updates and customer status page.
- **Subject Matter Expert (SME)**: Engineer(s) debugging and fixing the issue.

## 4. Response Process

### Phase 1: Detection & Analysis
- Alert received (PagerDuty/Slack).
- First responder verifies the incident.
- IC is assigned.
- Determine Severity (P0-P3).

### Phase 2: Containment & Eradication
- **Contain**: Stop the bleeding (e.g., block IP, roll back deployment, failover to DR region).
- **Eradicate**: Remove the root cause (patch vulnerability, restart service).
- Preserve evidence if security-related (logs, disk snapshots).

### Phase 3: Recovery
- Restore full service functionality.
- Verify system integrity via health checks and canary tests.
- Monitoring for recurrence.

### Phase 4: Post-Incident Activity
- **Retrospective**: Conduct a "Blameless Post-Mortem" within 24 hours (for P0/P1).
- **Report**: Create an RFO (Reason for Outage) document.
- **Action Items**: Create tickets to fix root cause and improve detection.

## 5. Communication Channels

- **Internal**: Slack channel `#incident-war-room`.
- **Public**: Status Page (`status.sapliy-cloud.com`) updates every 30 mins during P0.
- **Support**: Notify affected enterprise customers via email/intercom.

## 6. Testing
- This plan is tested via **Game Days** (simulated outages) once per quarter.
