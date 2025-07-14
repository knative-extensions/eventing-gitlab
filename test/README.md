# e2e test

This directory contains end-to-end integration tests for the GitLab eventing source. These tests verify the complete webhook integration by setting up a GitLabSource and simulating real GitLab webhook events.

## Prerequisites

- A Kubernetes cluster with Knative Serving and Eventing installed
- `kubectl` configured to access your cluster
- The GitLab eventing source controller deployed

## Local Setup with Kind

For local testing, you can use Knative Quickstart with Kind:

```bash
kn quickstart kind
```

## Deploy Test Resources

Deploy the required test resources in the following order:

```bash
# Deploy an event display service to receive webhook events
kubectl -n default apply -f samples/event-display.yaml

# Create the secret containing GitLab tokens
kubectl -n default apply -f samples/secret.yaml

# Deploy the GitLabSource
kubectl -n default apply -f samples/gitlabsource.yaml
```

## Wait for Resources to be Ready

Ensure all resources are ready before running tests:

```bash
# Wait for the GitLabSource to be ready
kubectl wait --for=condition=Ready gitlabsource/gitlabsource-sample -n default --timeout=300s

# Check the GitLabSource adapter service is created
kubectl get ksvc -n default | grep gitlabsource-sample
```

### Run Integration Tests Only

```bash
# Run all integration tests
go test -tags=integration ./test -v
```
