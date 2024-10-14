// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lookup

import (
	"fmt"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("active connections store test", func() {
	var (
		ipV4Addresses      = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0"), net.ParseIP("255.255.255.255"), net.ParseIP("192.168.123.45"), net.ParseIP("10.11.12.13")}
		shortIPV4Addresses = []net.IP{net.ParseIP("127.0.0.1").To4(), net.ParseIP("0.0.0.0").To4(), net.ParseIP("255.255.255.255").To4(), net.ParseIP("192.168.123.45").To4(), net.ParseIP("10.11.12.13").To4()}
		ipV6Addresses      = []net.IP{net.ParseIP("::1"), net.ParseIP("::"), net.ParseIP("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"), net.ParseIP("2001:db8:1234:5678:abcd:ef01:9876:5432"), net.ParseIP("fc00::7654")}

		lookupTable ActiveConnectionPairs
	)

	BeforeEach(func() {
		lookupTable = NewLookupTable()
	})

	DescribeTable("should add/remove connection data",
		func(src []net.IP, dst []net.IP, svcDst []net.IP, isAdd []bool, srcCheck []net.IP, dstCheck []net.IP, checkResults []bool) {
			Expect(len(src)).To(Equal(len(dst)))
			Expect(len(src)).To(Equal(len(svcDst)))
			Expect(len(src)).To(Equal(len(isAdd)))
			Expect(len(srcCheck)).To(Equal(len(dstCheck)))
			Expect(len(srcCheck)).To(Equal(len(checkResults)))
			for i := range src {
				if isAdd[i] {
					lookupTable.Add(src[i], dst[i], svcDst[i])
				} else {
					lookupTable.Remove(src[i], dst[i], svcDst[i])
				}
			}
			for i := range srcCheck {
				By(fmt.Sprintf("Checking connection %s->%s", srcCheck[i], dstCheck[i]))
				Expect(lookupTable.Exists(srcCheck[i], dstCheck[i])).To(Equal(checkResults[i]))
			}
		},

		Entry("single ipv4 connection",
			[]net.IP{ipV4Addresses[0]},
			[]net.IP{ipV4Addresses[1]},
			[]net.IP{ipV4Addresses[1]},
			[]bool{true},
			[]net.IP{ipV4Addresses[0], ipV4Addresses[0]},
			[]net.IP{ipV4Addresses[1], ipV4Addresses[0]},
			[]bool{true, false}),
		Entry("single (short) ipv4 connection",
			[]net.IP{shortIPV4Addresses[0]},
			[]net.IP{shortIPV4Addresses[1]},
			[]net.IP{shortIPV4Addresses[1]},
			[]bool{true},
			[]net.IP{shortIPV4Addresses[0], shortIPV4Addresses[0]},
			[]net.IP{shortIPV4Addresses[1], shortIPV4Addresses[0]},
			[]bool{true, false}),
		Entry("single ipv6 connection",
			[]net.IP{ipV6Addresses[0]},
			[]net.IP{ipV6Addresses[1]},
			[]net.IP{ipV6Addresses[1]},
			[]bool{true},
			[]net.IP{ipV6Addresses[0], ipV6Addresses[0]},
			[]net.IP{ipV6Addresses[1], ipV6Addresses[0]},
			[]bool{true, false}),
		Entry("multiple ipv4 connection",
			[]net.IP{ipV4Addresses[0], ipV4Addresses[1], ipV4Addresses[2], ipV4Addresses[3], ipV4Addresses[4]},
			[]net.IP{ipV4Addresses[1], ipV4Addresses[2], ipV4Addresses[3], ipV4Addresses[4], ipV4Addresses[0]},
			[]net.IP{ipV4Addresses[1], ipV4Addresses[2], ipV4Addresses[3], ipV4Addresses[4], ipV4Addresses[0]},
			[]bool{true, true, true, true, true},
			[]net.IP{ipV4Addresses[0], ipV4Addresses[0], ipV4Addresses[4]},
			[]net.IP{ipV4Addresses[1], ipV4Addresses[0], ipV4Addresses[0]},
			[]bool{true, false, true}),
		Entry("multiple (short) ipv4 connection",
			[]net.IP{shortIPV4Addresses[0], shortIPV4Addresses[1], shortIPV4Addresses[2], shortIPV4Addresses[3], shortIPV4Addresses[4]},
			[]net.IP{shortIPV4Addresses[1], shortIPV4Addresses[2], shortIPV4Addresses[3], shortIPV4Addresses[4], shortIPV4Addresses[0]},
			[]net.IP{shortIPV4Addresses[1], shortIPV4Addresses[2], shortIPV4Addresses[3], shortIPV4Addresses[4], shortIPV4Addresses[0]},
			[]bool{true, true, true, true, true},
			[]net.IP{shortIPV4Addresses[0], shortIPV4Addresses[0], shortIPV4Addresses[4]},
			[]net.IP{shortIPV4Addresses[1], shortIPV4Addresses[0], shortIPV4Addresses[0]},
			[]bool{true, false, true}),
		Entry("multiple ipv6 connection",
			[]net.IP{ipV6Addresses[0], ipV6Addresses[1], ipV6Addresses[2], ipV6Addresses[3], ipV6Addresses[4]},
			[]net.IP{ipV6Addresses[1], ipV6Addresses[2], ipV6Addresses[3], ipV6Addresses[4], ipV6Addresses[0]},
			[]net.IP{ipV6Addresses[1], ipV6Addresses[2], ipV6Addresses[3], ipV6Addresses[4], ipV6Addresses[0]},
			[]bool{true, true, true, true, true},
			[]net.IP{ipV6Addresses[0], ipV6Addresses[0], ipV6Addresses[4]},
			[]net.IP{ipV6Addresses[1], ipV6Addresses[0], ipV6Addresses[0]},
			[]bool{true, false, true}),
		Entry("multiple ipv4 connection with removals",
			[]net.IP{ipV4Addresses[0], ipV4Addresses[1], ipV4Addresses[0], ipV4Addresses[0], ipV4Addresses[0]},
			[]net.IP{ipV4Addresses[1], ipV4Addresses[2], ipV4Addresses[1], ipV4Addresses[1], ipV4Addresses[1]},
			[]net.IP{ipV4Addresses[1], ipV4Addresses[2], ipV4Addresses[1], ipV4Addresses[1], ipV4Addresses[1]},
			[]bool{true, true, true, false, false},
			[]net.IP{ipV4Addresses[0], ipV4Addresses[0], ipV4Addresses[1]},
			[]net.IP{ipV4Addresses[1], ipV4Addresses[0], ipV4Addresses[2]},
			[]bool{false, false, true}),
		Entry("multiple ipv4 connection with removals",
			[]net.IP{shortIPV4Addresses[0], shortIPV4Addresses[1], shortIPV4Addresses[0], shortIPV4Addresses[0], shortIPV4Addresses[0]},
			[]net.IP{shortIPV4Addresses[1], shortIPV4Addresses[2], shortIPV4Addresses[1], shortIPV4Addresses[1], shortIPV4Addresses[1]},
			[]net.IP{shortIPV4Addresses[1], shortIPV4Addresses[2], shortIPV4Addresses[1], shortIPV4Addresses[1], shortIPV4Addresses[1]},
			[]bool{true, true, true, false, false},
			[]net.IP{shortIPV4Addresses[0], shortIPV4Addresses[0], shortIPV4Addresses[1]},
			[]net.IP{shortIPV4Addresses[1], shortIPV4Addresses[0], shortIPV4Addresses[2]},
			[]bool{false, false, true}),
		Entry("multiple ipv6 connection with removals",
			[]net.IP{ipV6Addresses[0], ipV6Addresses[1], ipV6Addresses[0], ipV6Addresses[0], ipV6Addresses[0]},
			[]net.IP{ipV6Addresses[1], ipV6Addresses[2], ipV6Addresses[1], ipV6Addresses[1], ipV6Addresses[1]},
			[]net.IP{ipV6Addresses[1], ipV6Addresses[2], ipV6Addresses[1], ipV6Addresses[1], ipV6Addresses[1]},
			[]bool{true, true, true, false, false},
			[]net.IP{ipV6Addresses[0], ipV6Addresses[0], ipV6Addresses[1]},
			[]net.IP{ipV6Addresses[1], ipV6Addresses[0], ipV6Addresses[2]},
			[]bool{false, false, true}),
		Entry("multiple ipv4 connection with removals and services",
			[]net.IP{ipV4Addresses[0], ipV4Addresses[1], ipV4Addresses[0], ipV4Addresses[0], ipV4Addresses[0]},
			[]net.IP{ipV4Addresses[1], ipV4Addresses[2], ipV4Addresses[1], ipV4Addresses[1], ipV4Addresses[1]},
			[]net.IP{ipV4Addresses[2], ipV4Addresses[3], ipV4Addresses[2], ipV4Addresses[2], ipV4Addresses[2]},
			[]bool{true, true, true, false, false},
			[]net.IP{ipV4Addresses[0], ipV4Addresses[0], ipV4Addresses[1], ipV4Addresses[0], ipV4Addresses[1]},
			[]net.IP{ipV4Addresses[1], ipV4Addresses[0], ipV4Addresses[2], ipV4Addresses[2], ipV4Addresses[3]},
			[]bool{false, false, true, false, true}),
		Entry("multiple ipv4 connection with removals and services",
			[]net.IP{shortIPV4Addresses[0], shortIPV4Addresses[1], shortIPV4Addresses[0], shortIPV4Addresses[0], shortIPV4Addresses[0]},
			[]net.IP{shortIPV4Addresses[1], shortIPV4Addresses[2], shortIPV4Addresses[1], shortIPV4Addresses[1], shortIPV4Addresses[1]},
			[]net.IP{shortIPV4Addresses[2], shortIPV4Addresses[3], shortIPV4Addresses[2], shortIPV4Addresses[2], shortIPV4Addresses[2]},
			[]bool{true, true, true, false, false},
			[]net.IP{shortIPV4Addresses[0], shortIPV4Addresses[0], shortIPV4Addresses[1], shortIPV4Addresses[0], shortIPV4Addresses[1]},
			[]net.IP{shortIPV4Addresses[1], shortIPV4Addresses[0], shortIPV4Addresses[2], shortIPV4Addresses[2], shortIPV4Addresses[3]},
			[]bool{false, false, true, false, true}),
		Entry("multiple ipv6 connection with removals and services",
			[]net.IP{ipV6Addresses[0], ipV6Addresses[1], ipV6Addresses[0], ipV6Addresses[0], ipV6Addresses[0]},
			[]net.IP{ipV6Addresses[1], ipV6Addresses[2], ipV6Addresses[1], ipV6Addresses[1], ipV6Addresses[1]},
			[]net.IP{ipV6Addresses[2], ipV6Addresses[3], ipV6Addresses[2], ipV6Addresses[2], ipV6Addresses[2]},
			[]bool{true, true, true, false, false},
			[]net.IP{ipV6Addresses[0], ipV6Addresses[0], ipV6Addresses[1], ipV6Addresses[0], ipV6Addresses[1]},
			[]net.IP{ipV6Addresses[1], ipV6Addresses[0], ipV6Addresses[2], ipV6Addresses[2], ipV6Addresses[3]},
			[]bool{false, false, true, false, true}),
	)
})
