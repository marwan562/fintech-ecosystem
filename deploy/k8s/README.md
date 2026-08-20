# Kubernetes Deployment Guide

This directory contains the Kubernetes manifests to deploy the entire Sapliy Ecosystem.

## Prerequisites

- Not running existing docker-compose (ports might conflict if using LoadBalancer on localhost)
- `kubectl` installed
- A Kubernetes cluster (Minikube, Kind, or robust cloud provider)

## Quick Start (Local)

We provide a helper script to apply everything in the correct order:

```bash
chmod +x apply.sh
./apply.sh
```

## Manual Deployment

1. **Create Namespace**:
   ```bash
   kubectl apply -f namespace.yaml
   ```

2. **Deploy Infrastructure** (Postgres, Redis, RabbitMQ, Redpanda):
   ```bash
   kubectl apply -f infrastructure.yaml
   ```
   *Wait for pods to be ready.*

3. **Deploy Microservices**:
   ```bash
   kubectl apply -f auth.yaml
   kubectl apply -f payments.yaml
   kubectl apply -f ledger.yaml
   kubectl apply -f gateway.yaml
   kubectl apply -f notifications.yaml
   kubectl apply -f fraud.yaml
   kubectl apply -f reconciler.yaml
   ```

## Architecture Notes

- **Postgres**: Deployed as a single instance with multiple databases (`microservices`, `payments`, `ledger`) created via ConfigMap init script to save resources.
- **Redpanda**: Deployed as a single-node StatefulSet.
- **Microservices**: Configured with `imagePullPolicy: IfNotPresent` for local development. Ensure you build the images locally across your cluster nodes (content loading in Kind/Minikube) or push to a registry.

## Local Image Loading (Kind/Minikube)

If using Kind:
```bash
kind load docker-image microservice:latest
```

If using Minikube:
```bash
eval $(minikube docker-env)
make build # Rebuild inside minikube's docker
```
