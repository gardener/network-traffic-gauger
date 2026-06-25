# Local Testing Guide

## Prerequisites

- Docker Desktop (or colima) running on macOS/Linux
- [kind](https://kind.sigs.k8s.io/) installed
- kubectl configured

## Build

```bash
make docker-images IMAGE_TAG=dev
```

> Override `GOARCH` if cross-compiling: `make docker-images IMAGE_TAG=dev GOARCH=arm64`

## Create a kind cluster and load the image

```bash
kind create cluster --name gauger-test
kind load docker-image europe-docker.pkg.dev/gardener-project/snapshots/gardener/network-traffic-gauger:dev --name gauger-test
```

## Deploy

The example DaemonSet in [`examples/test-daemonset.yaml`](../examples/test-daemonset.yaml)
runs NTG with `--extract-local-ranges-from-kubernetes` enabled so that the
node's pod CIDR is correctly treated as local traffic.

```bash
kubectl apply -f examples/test-daemonset.yaml
kubectl -n kube-system rollout status daemonset/network-traffic-gauger --timeout=60s
```

## Verify

Check that the init container enabled conntrack accounting:

```bash
kubectl -n kube-system logs -l app=network-traffic-gauger -c setup
```

Expected output:

```text
Checking and enabling netfilter accounting/timestamps (if required)...
Checking netfilter accounting setting in '/proc/sys/net/netfilter/nf_conntrack_acct'...
Netfilter accounting already enabled.
```

Check that the agent started:

```bash
kubectl -n kube-system logs -l app=network-traffic-gauger -c agent
```

Expected output includes:

```text
Initializing cluster information...
Using the following network ranges as cluster: [10.244.0.0/16]
Determined local ip address range from kubernetes: [10.244.0.0/24]
Serving metrics on port 16160...
Running network traffic measurement agent...
```

## Scrape metrics

Port-forward to the NTG pod:

```bash
kubectl -n kube-system port-forward ds/network-traffic-gauger 16160:16160
```

In another terminal:

```bash
curl -s localhost:16160/metrics | grep '^network_'
```

You should see counters like:

```text
network_flow_total{dst="...",src="...",type="local"} 21
network_receive_bytes_total{dst="...",src="...",type="local"} 14847
network_transmit_bytes_total{dst="...",src="...",type="local"} 9879
```

Traffic types are `local`, `cluster`, `link-local`, or `internet` depending on
source/destination IP ranges.

## Generate traffic

Run a pod that makes requests to produce observable flows:

```bash
kubectl run traffic-gen --rm -it --restart=Never --image=curlimages/curl -- \
  sh -c 'for i in $(seq 1 10); do curl -s -o /dev/null https://kubernetes.default; done'
```

Then re-scrape metrics to see additional flows. Traffic from a local pod to the
Kubernetes API server (node IP) should appear as `local`.

## Cleanup

```bash
kind delete cluster --name gauger-test
```
