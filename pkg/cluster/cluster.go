// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/gardener/network-traffic-gauger/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ClusterInfo interface {
	Update(ctx context.Context) error
	IsLocalAddress(ip netip.Addr) bool
	IsInClusterRange(ip netip.Addr) bool
}

type clusterInfo struct {
	useKubernetes         bool
	kubeconfigPath        string
	nodeName              string
	localAddresses        map[netip.Addr]bool
	localRanges           []netip.Prefix
	localKubernetesRanges []netip.Prefix
	clusterRanges         []netip.Prefix
}

func NewClusterInfo(localRanges []string, clusterRanges []string, useKubernetes bool, kubeconfigPath string, nodeName string) (ClusterInfo, error) {
	if useKubernetes && len(nodeName) == 0 {
		return nil, fmt.Errorf("node name cannot be empty when using kubernetes to retrieve local ranges")
	}
	localPrefixes, err := parsePrefixes(localRanges)
	if err != nil {
		return nil, err
	}
	clusterPrefixes, err := parsePrefixes(clusterRanges)
	if err != nil {
		return nil, err
	}
	log := logrus.WithField("component", "cluster-info")
	if len(localPrefixes) > 0 {
		log.Infof("Using the following network ranges as local: %v", localPrefixes)
	}
	log.Infof("Using the following network ranges as cluster: %v", clusterPrefixes)
	return &clusterInfo{
		useKubernetes:  useKubernetes,
		kubeconfigPath: kubeconfigPath,
		nodeName:       nodeName,
		localRanges:    localPrefixes,
		clusterRanges:  clusterPrefixes,
	}, nil
}

func parsePrefixes(rawPrefixes []string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, network := range rawPrefixes {
		prefix, err := netip.ParsePrefix(network)
		if err != nil {
			return nil, fmt.Errorf("parsing cluster range '%s' failed: %w", network, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func (ci *clusterInfo) Update(ctx context.Context) error {
	localKubernetesRanges := []netip.Prefix{}
	if ci.useKubernetes {
		cfg, err := clientcmd.BuildConfigFromFlags("", ci.kubeconfigPath)
		if err != nil {
			return fmt.Errorf("building kubeconfig failed: %w", err)
		}
		cs, err := clientset.NewForConfig(cfg)
		if err != nil {
			return fmt.Errorf("creating kubernetes clientset failed: %w", err)
		}
		node, err := cs.CoreV1().Nodes().Get(ctx, ci.nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("getting node '%s' failed: %w", ci.nodeName, err)
		}
		localKubernetesRanges, err = parsePrefixes(node.Spec.PodCIDRs)
		if err != nil {
			return fmt.Errorf("failed to parse node.spec.podCIDRs ('%v') of node '%s': %w", node.Spec.PodCIDRs, ci.nodeName, err)
		}
	}
	deviceAddresses, err := netlink.AddrList(nil, 0)
	if err != nil {
		return fmt.Errorf("retrieving local addresses failed: %w", err)
	}
	localIPs := map[netip.Addr]bool{}
	for _, address := range deviceAddresses {
		if address.IP.IsLoopback() || address.IP.IsLinkLocalUnicast() || address.IP.IsLinkLocalMulticast() {
			continue
		}
		ip, ok := utils.ConvertIP(&address.IP)
		if !ok {
			return fmt.Errorf("error while converting local device ip address '%s' during cluster info update: expected byte length 4 or 16, but got %d", address.IP.String(), len(address.IP))
		}
		localIPs[ip] = true
	}
	ci.localKubernetesRanges = localKubernetesRanges
	ci.localAddresses = localIPs
	log := logrus.WithField("component", "cluster-info")
	if len(localKubernetesRanges) > 0 {
		log.Infof("Determined local ip address range from kubernetes: %v", localKubernetesRanges)
	}
	log.Infof("Determined the following ip addresses to be local addresses: %v", localIPs)
	return nil
}

func (ci *clusterInfo) IsLocalAddress(ip netip.Addr) bool {
	_, exists := ci.localAddresses[ip]
	if exists {
		return true
	}
	return isInRange(ci.localRanges, ip) || isInRange(ci.localKubernetesRanges, ip)
}

func (ci *clusterInfo) IsInClusterRange(ip netip.Addr) bool {
	return isInRange(ci.clusterRanges, ip)
}

func isInRange(prefixes []netip.Prefix, ip netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}
