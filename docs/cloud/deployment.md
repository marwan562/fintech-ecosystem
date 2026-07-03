# Cloud Deployment Guide

This guide details how to deploy the Fintech Ecosystem infrastructure to AWS using Terraform and Helm.

## Prerequisites

- **AWS Account** with Administrator privileges.
- **Terraform** v1.5+ installed.
- **AWS CLI** v2+ configured.
- **kubectl** and **helm** v3+ installed.
- **Domain Name** managed in Route53.

## Infrastructure Provisioning (Terraform)

The `deploy/cloud/terraform` directory contains the Infrastructure as Code definitions.

### 1. Initialize
Navigate to the terraform directory and initialize the modules.

```bash
cd deploy/cloud/terraform
terraform init
```

### 2. Configure Variables
Create a `terraform.tfvars` file to customize your deployment:

```hcl
region          = "us-east-1"
environment     = "production"
cluster_name    = "fintech-cloud-prod"
vpc_cidr        = "10.0.0.0/16"
db_instance_class = "db.t4g.large"
allowed_cidrs   = ["1.2.3.4/32"] # Admin VPN IPs
```

### 3. Plan and Apply
Review changes and provision infrastructure.

```bash
terraform plan -out=tfplan
terraform apply tfplan
```

*This process takes 15-20 minutes to provision EKS, RDS, and MSK.*

## Application Deployment (Helm)

Once infrastructure is ready, deploy the application stack.

### 1. Update Kubeconfig
Connect to the new EKS cluster.

```bash
aws eks update-kubeconfig --region us-east-1 --name fintech-cloud-prod
```

### 2. Configure Secrets
We use External Secrets Operator to fetch secrets from AWS Secrets Manager. Ensure the following secrets are created in AWS:

- `fintech/prod/database-credentials`
- `fintech/prod/kafka-sasl`
- `fintech/prod/jwt-secrets`

### 3. Deploy Helm Chart
Deploy the `sapliy-core` chart.

```bash
helm upgrade --install sapliy-core ./deploy/helm/sapliy-core \
  --namespace sapliy-core \
  --create-namespace \
  --values values-prod.yaml
```

## Maintenance & Operations

### Database Migrations
Migrations run automatically as Kubernetes Jobs on Helm install/upgrade. To run manually:

```bash
kubectl create job --from=cronjob/migration-runner migration-manual-001
```

### Scaling
- **Compute**: The EKS Cluster Autoscaler handles node scaling. HPA handles pod scaling based on CPU/Memory.
- **Database**: Modify `terraform.tfvars` instance size and apply, or use AWS Console for storage scaling (zero-downtime).

### Backups
- **RDS**: Automated daily snapshots + Point-in-Time Recovery (PITR) enabled (7 days retention default).
- **Secrets**: Not backed up (infrastructure code is the source of truth).
- **Logs**: Retained in CloudWatch Logs for 365 days.

## Disaster Recovery

1.  **Region Failure**: Re-run Terraform in a secondary region (e.g., `us-west-2`).
2.  **Data Restore**: Restore RDS from the latest snapshot to the new region.
3.  **DNS Failover**: Update Route53 records to point to the new region's ALB.
