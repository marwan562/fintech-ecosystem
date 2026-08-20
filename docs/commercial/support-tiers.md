# Support Tiers & SLA Packages

This document outlines the commercial support offerings for the Sapliy Ecosystem platform. Whether you're a startup running self-hosted or an enterprise using our managed cloud, we offer support packages to match your needs.

## Overview

We offer three tiers of support, each designed for different stages of business growth:

| Tier | Best For | Response Time | Price |
|------|----------|---------------|-------|
| **Essential** | Self-hosted users, hobbyists | 48h (Email) | Free |
| **Professional** | Growing startups | 24h (Priority Email), 4h (Critical) | $499/mo |
| **Enterprise** | Mission-critical deployments | 1h (Critical), 24/7 Phone | $1,999/mo |

---

## Tier Details

### Essential (Free)

**Best for**: Developers learning the platform, small projects, and self-hosted users.

**Features**:
- Community forum access
- Email support (48-hour response time)
- Full documentation access
- Standard business hours support (9 AM - 6 PM EST, Mon-Fri)

**SLA Commitments**:
| Priority | First Response |
|----------|---------------|
| Critical | 48 hours |
| High | 48 hours |
| Normal | 48 hours |
| Low | 48 hours |

---

### Professional ($499/month or $4,990/year)

**Best for**: Growing startups, teams scaling their payment infrastructure, and businesses requiring reliable support.

**Features**:
- Priority email support (24-hour response time)
- Phone support during business hours
- Dedicated Slack channel for your team
- Quarterly business reviews
- 99.9% uptime SLA guarantee
- Access to beta features
- Priority in community forums

**SLA Commitments**:
| Priority | First Response | Resolution Target |
|----------|---------------|-------------------|
| Critical | 4 hours | 24 hours |
| High | 8 hours | 48 hours |
| Normal | 24 hours | 72 hours |
| Low | 48 hours | Best effort |

**Critical Issues Include**:
- Complete service outage
- Data corruption or loss risk
- Security vulnerabilities

---

### Enterprise ($1,999/month or $19,990/year)

**Best for**: Mission-critical deployments, financial institutions, and businesses requiring guaranteed SLAs.

**Features**:
- 24/7 phone and email support
- 1-hour response for critical issues
- Dedicated support engineer
- Custom SLA agreements available
- Executive escalation path
- 99.99% uptime SLA guarantee
- Priority bugfix queue
- Direct access to engineering team
- Dedicated Slack channel with real-time monitoring
- Monthly account reviews
- Custom training sessions

**SLA Commitments**:
| Priority | First Response | Resolution Target |
|----------|---------------|-------------------|
| Critical | 1 hour | 8 hours |
| High | 2 hours | 24 hours |
| Normal | 8 hours | 48 hours |
| Low | 24 hours | 72 hours |

**Additional Guarantees**:
- Maximum 4 critical incidents per year (compensation for excess)
- Dedicated escalation hotline
- Post-incident review within 72 hours

---

## Escalation Procedures

### Standard Escalation Path

1. **Level 1**: Support Engineer (Initial Response)
2. **Level 2**: Senior Support Engineer (Complex Issues)
3. **Level 3**: Engineering Team Lead (Product-level Issues)
4. **Level 4**: VP of Engineering (Critical Escalations)

### Enterprise-Only Escalation

Enterprise customers have access to:
- **Direct Engineering Contact**: Bypass Level 1 for known complex issues
- **Executive Hotline**: Direct line to leadership for business-critical situations
- **On-call Engineer**: 24/7 access to on-call rotation

---

## Ticket Priority Guidelines

### Critical
- Complete platform outage
- Security breach or vulnerability
- Data loss or corruption
- Payment processing failure affecting all transactions

### High
- Partial service degradation
- Performance issues affecting multiple users
- Integration failures
- Billing discrepancies

### Normal
- Feature not working as expected
- Configuration questions
- Best practices guidance
- Non-urgent feature requests

### Low
- Documentation clarifications
- General questions
- Enhancement suggestions

---

## SLA Breach Compensation

### Professional Tier
- Response SLA breach: 10% credit on monthly bill per breach (max 50%)
- Uptime SLA breach: 10% credit per 0.1% below 99.9% (max 100%)

### Enterprise Tier
- Response SLA breach: 15% credit on monthly bill per breach (max 100%)
- Uptime SLA breach: 25% credit per 0.01% below 99.99% (max 100%)
- Critical incident excess: $1,000 credit per incident over 4/year

---

## Getting Started

### Sign Up for Support

1. Visit your **Organization Settings** > **Support Plan**
2. Select your desired tier
3. Complete payment information
4. You're covered immediately

### Creating a Support Ticket

**Via API**:
```bash
curl -X POST https://api.sapliy-core.com/v1/support/tickets \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "subject": "Payment processing issue",
    "description": "Detailed description of the issue...",
    "priority": "high",
    "category": "technical"
  }'
```

**Via Dashboard**:
1. Navigate to **Support** > **Create Ticket**
2. Fill in the required details
3. Select appropriate priority
4. Submit

### Contacting Support

| Tier | Email | Phone |
|------|-------|-------|
| Essential | support@sapliy-core.com | — |
| Professional | priority-support@sapliy-core.com | +1 (888) 555-0123 |
| Enterprise | enterprise-support@sapliy-core.com | +1 (888) 555-0199 (24/7) |

---

## Frequently Asked Questions

**Q: Can I upgrade my tier mid-billing cycle?**
A: Yes! Upgrades take effect immediately. You'll be charged a prorated amount for the remainder of your billing cycle.

**Q: What happens when my support contract expires?**
A: You'll automatically move to the Essential tier. Outstanding tickets remain open until resolved.

**Q: Are there volume discounts for multiple years?**
A: Yes. Contact sales@sapliy-core.com for multi-year agreements.

**Q: Can I request specific engineers?**
A: Enterprise customers can request a dedicated support engineer who becomes familiar with their implementation.

---

## Contact Sales

For custom requirements or enterprise agreements:
- **Email**: sales@sapliy-core.com
- **Phone**: +1 (888) 555-SALE
- **Schedule a Call**: [calendly.com/sapliy-sales](https://calendly.com/sapliy-sales)
