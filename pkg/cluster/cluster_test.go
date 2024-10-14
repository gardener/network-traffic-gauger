// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"fmt"
	"net"

	"github.com/gardener/network-traffic-gauger/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("cluster information test", func() {
	var (
		localRanges  = []string{"10.0.0.0/24", "2001:db8::/64"}
		clusterRange = []string{"10.0.0.0/16", "2001:db8::/32"}
		clusterInfo  Info
	)

	BeforeEach(func() {
		var err error
		clusterInfo, err = NewInfo(localRanges, clusterRange, false, "", "")
		Expect(err).To(BeNil())
	})

	DescribeTable("should add/remove connection data",
		func(ips []net.IP, isLocal []bool, isCluster []bool) {
			for i, ip := range ips {
				By(fmt.Sprintf("Checking IP address '%s'", ip.String()))
				netip, ok := utils.ConvertIP(&ip)
				Expect(ok).To(BeTrue())
				Expect(clusterInfo.IsLocalAddress(netip)).To(Equal(isLocal[i]))
				Expect(clusterInfo.IsInClusterRange(netip)).To(Equal(isCluster[i]))
			}
		},

		Entry("ipv4 checks",
			[]net.IP{net.ParseIP("10.0.0.0"), net.ParseIP("10.0.0.255"), net.ParseIP("10.0.1.0"), net.ParseIP("10.0.255.255"), net.ParseIP("10.1.0.0"), net.ParseIP("192.168.0.0")},
			[]bool{true, true, false, false, false, false},
			[]bool{true, true, true, true, false, false}),
		Entry("ipv6 checks",
			[]net.IP{net.ParseIP("2001:db8::"), net.ParseIP("2001:db8:0:0:ffff:ffff:ffff:ffff"), net.ParseIP("2001:db8:0:1::"), net.ParseIP("2001:db8:ffff:ffff:ffff:ffff:ffff:ffff"), net.ParseIP("2001:db9::"), net.ParseIP("fe80:abcd::")},
			[]bool{true, true, false, false, false, false},
			[]bool{true, true, true, true, false, false}),
	)
})
