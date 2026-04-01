# Local Testing Guide

## Prerequisites

- Docker Desktop (or colima) running on macOS/Linux
- [kind](https://kind.sigs.k8s.io/) installed
- kubectl configured

## Build

```bash
docker build --platform linux/arm64 -t network-traffic-gauger:dev .
```

> For AMD64 hosts, use `--platform linux/amd64`.

## Create a kind cluster and load the image

```bash
kind create cluster --name gauger-test
kind load docker-image network-traffic-gauger:dev --name gauger-test
```

## Deploy

```bash
kubectl apply -f hack/test-daemonset.yaml
kubectl -n kube-system rollout status daemonset/network-traffic-gauger --timeout=60s
```

## Verify

Check that the init container enabled conntrack accounting and the agent started:

```bash
kubectl -n kube-system logs -l app=network-traffic-gauger -c setup
kubectl -n kube-system logs -l app=network-traffic-gauger -c agent
```

Expected agent log output includes:
```
Serving metrics on port 16160...
Running network traffic measurement agent...
```

## Scrape metrics

The metrics endpoint is on port 16160 of the host network. Get the node IP and
curl from a temporary pod:

```bash
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
kubectl run curl-test --rm -it --restart=Never --image=curlimages/curl -- \
  curl -s "http://${NODE_IP}:16160/metrics" | grep '^network_'
```

You should see counters like:
```
network_flow_total{dst="...",src="...",type="local"} 21
network_receive_bytes_total{dst="...",src="...",type="local"} 14847
network_transmit_bytes_total{dst="...",src="...",type="local"} 9879
```

Traffic types are `local`, `cluster`, `link-local`, or `internet` depending on
source/destination IP ranges.

## Generate more traffic

Run a pod that makes requests to produce observable flows:

```bash
kubectl run traffic-gen --rm -it --restart=Never --image=curlimages/curl -- \
  sh -c 'for i in $(seq 1 10); do curl -s -o /dev/null https://kubernetes.default; done'
```

Then re-scrape metrics to see additional `internet`-typed flows from the pod
CIDR to the node.

## Cleanup

```bash
kind delete cluster --name gauger-test
```
