#!/bin/bash

echo "🚀 Deploying Sapliy Ecosystem to Kubernetes..."

# Create Namespace
kubectl apply -f namespace.yaml

echo "📦 deploying infrastructure..."
kubectl apply -f infrastructure.yaml

echo "⏳ Waiting for Infrastructure to be ready..."
kubectl wait --namespace sapliy-ecosystem \
  --for=condition=ready pod \
  --selector=app=postgres \
  --timeout=90s

kubectl wait --namespace sapliy-ecosystem \
  --for=condition=ready pod \
  --selector=app=redis \
  --timeout=90s

echo "🔎 Deploying Observability (Jaeger)..."
kubectl apply -f observability.yaml

kubectl wait --namespace sapliy-ecosystem \
  --for=condition=ready pod \
  --selector=app=jaeger \
  --timeout=90s

echo "🦄 Deploying Services..."
kubectl apply -f auth.yaml
kubectl apply -f payments.yaml
kubectl apply -f ledger.yaml
kubectl apply -f gateway.yaml
kubectl apply -f notifications.yaml
kubectl apply -f fraud.yaml
kubectl apply -f reconciler.yaml

echo "✅ Deployment requests sent. Check status with: kubectl get pods -n sapliy-ecosystem"
