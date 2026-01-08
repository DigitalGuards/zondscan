#!/bin/bash
# Deploy to Kubernetes cluster
# Usage: ./scripts/k8s-deploy.sh [--delete]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
K8S_DIR="$PROJECT_ROOT/k8s"

# Parse arguments
if [[ "$1" == "--delete" ]]; then
    echo "========================================"
    echo "Deleting QRL Zond Explorer from Kubernetes"
    echo "========================================"
    echo ""

    echo "Deleting ingress..."
    kubectl delete -f "$K8S_DIR/ingress.yaml" --ignore-not-found

    echo "Deleting syncer..."
    kubectl delete -f "$K8S_DIR/syncer/" --ignore-not-found

    echo "Deleting frontend..."
    kubectl delete -f "$K8S_DIR/frontend/" --ignore-not-found

    echo "Deleting backend..."
    kubectl delete -f "$K8S_DIR/backend/" --ignore-not-found

    echo "Deleting MongoDB..."
    kubectl delete -f "$K8S_DIR/mongodb/" --ignore-not-found

    echo "Deleting secrets..."
    kubectl delete -f "$K8S_DIR/secrets.yaml" --ignore-not-found

    echo "Deleting configmap..."
    kubectl delete -f "$K8S_DIR/configmap.yaml" --ignore-not-found

    echo "Deleting namespace..."
    kubectl delete -f "$K8S_DIR/namespace.yaml" --ignore-not-found

    echo ""
    echo "========================================"
    echo "Deletion complete!"
    echo "========================================"
    exit 0
fi

echo "========================================"
echo "Deploying QRL Zond Explorer to Kubernetes"
echo "========================================"
echo ""

# Create namespace
echo "[1/8] Creating namespace..."
kubectl apply -f "$K8S_DIR/namespace.yaml"

# Create configmap
echo "[2/8] Creating configmap..."
kubectl apply -f "$K8S_DIR/configmap.yaml"

# Create secrets
echo "[3/8] Creating secrets..."
kubectl apply -f "$K8S_DIR/secrets.yaml"

# Deploy MongoDB
echo "[4/8] Deploying MongoDB..."
kubectl apply -f "$K8S_DIR/mongodb/"

# Wait for MongoDB to be ready
echo "Waiting for MongoDB to be ready..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=mongodb -n zond-explorer --timeout=120s

# Deploy backend
echo "[5/8] Deploying backend..."
kubectl apply -f "$K8S_DIR/backend/"

# Deploy frontend
echo "[6/8] Deploying frontend..."
kubectl apply -f "$K8S_DIR/frontend/"

# Deploy syncer
echo "[7/8] Deploying syncer..."
kubectl apply -f "$K8S_DIR/syncer/"

# Deploy ingress
echo "[8/8] Deploying ingress..."
kubectl apply -f "$K8S_DIR/ingress.yaml"

echo ""
echo "========================================"
echo "Deployment complete!"
echo "========================================"
echo ""
echo "Check deployment status:"
echo "  kubectl get pods -n zond-explorer"
echo ""
echo "Check services:"
echo "  kubectl get svc -n zond-explorer"
echo ""
echo "View logs:"
echo "  kubectl logs -f deployment/backend -n zond-explorer"
echo "  kubectl logs -f deployment/frontend -n zond-explorer"
echo "  kubectl logs -f deployment/syncer -n zond-explorer"
