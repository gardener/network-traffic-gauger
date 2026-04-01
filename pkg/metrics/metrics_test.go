// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"net/netip"
	"testing"

	"github.com/gardener/network-traffic-gauger/pkg/cluster"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

var _ = Describe("determineConnectionType", func() {
	var ms *metricsServer

	BeforeEach(func() {
		// Realistic single-node setup:
		// Node IP: 10.0.0.1/32
		// Local pod range: 10.244.0.0/24 (this node's podCIDR)
		// Cluster ranges: 10.0.0.0/16 (nodes), 10.244.0.0/16 (all pods), 10.96.0.0/12 (services)
		info, err := cluster.NewInfo(
			[]string{"10.0.0.1/32", "10.244.0.0/24"},
			[]string{"10.0.0.0/16", "10.244.0.0/16", "10.96.0.0/12"},
			false, "", "",
		)
		Expect(err).NotTo(HaveOccurred())
		ms = &metricsServer{clusterInfo: info}
	})

	DescribeTable("should classify connections correctly",
		func(src, dst string, expectedType string) {
			srcIP := netip.MustParseAddr(src)
			dstIP := netip.MustParseAddr(dst)
			Expect(ms.determineConnectionType(srcIP, dstIP)).To(Equal(expectedType))
		},

		// local: both sides in local ranges (node IP + local pod CIDR)
		Entry("local pod to local pod", "10.244.0.5", "10.244.0.9", "local"),
		Entry("node to local pod", "10.0.0.1", "10.244.0.5", "local"),
		Entry("local pod to node", "10.244.0.5", "10.0.0.1", "local"),

		// cluster: one side local, other side in cluster range (remote node/pod)
		Entry("local pod to remote pod", "10.244.0.5", "10.244.1.5", "cluster"),
		Entry("remote pod to local pod", "10.244.1.5", "10.244.0.5", "cluster"),
		Entry("node to remote pod", "10.0.0.1", "10.244.1.5", "cluster"),
		Entry("local pod to remote node", "10.244.0.5", "10.0.1.1", "cluster"),
		Entry("node to service IP", "10.0.0.1", "10.96.0.1", "cluster"),

		// internet: one or both sides outside cluster ranges
		Entry("node to external", "10.0.0.1", "8.8.8.8", "internet"),
		Entry("external to node", "8.8.8.8", "10.0.0.1", "internet"),
		Entry("local pod to external", "10.244.0.5", "8.8.8.8", "internet"),
		Entry("external to external", "8.8.8.8", "1.1.1.1", "internet"),

		// link-local
		Entry("link-local src", "169.254.1.1", "10.244.0.5", "link-local"),
		Entry("link-local dst", "10.244.0.5", "169.254.1.1", "link-local"),
	)
})
