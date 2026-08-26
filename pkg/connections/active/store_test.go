// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package active

import (
	"net"

	"github.com/gardener/network-traffic-gauger/pkg/connections/lookup"

	"github.com/florianl/go-conntrack"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("active connections store test", func() {
	var (
		ipV4Addresses      = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0"), net.ParseIP("255.255.255.255"), net.ParseIP("192.168.123.45"), net.ParseIP("10.11.12.13")}
		shortIPV4Addresses = []net.IP{net.ParseIP("127.0.0.1").To4(), net.ParseIP("0.0.0.0").To4(), net.ParseIP("255.255.255.255").To4(), net.ParseIP("192.168.123.45").To4(), net.ParseIP("10.11.12.13").To4()}
		ipV6Addresses      = []net.IP{net.ParseIP("::1"), net.ParseIP("::"), net.ParseIP("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"), net.ParseIP("2001:db8:1234:5678:abcd:ef01:9876:5432"), net.ParseIP("fc00::7654")}

		dummyPort     uint16 = 12345
		dummyProtocol uint8  = 123
		dummyProto           = &conntrack.ProtoTuple{SrcPort: &dummyPort, DstPort: &dummyPort, Number: &dummyProtocol}
		connID1       uint32 = 1
		connID2       uint32 = 2
		connID3       uint32 = 3

		store Store
	)

	BeforeEach(func() {
		store = NewStore(lookup.NewLookupTable(), nil, 1024, false, false)
	})

	DescribeTable("should store/update connection data",
		func(flows []conntrack.Con, closed []bool, expectedCount int, expectedServiceCount int) {
			Expect(len(flows)).To(Equal(len(closed)))
			for i, c := range flows {
				store.HandleConnection(&c, "test", closed[i], nil)
			}
			count := 0
			Expect(store.IterateConnections(func(_, _ *net.IP, _, _, _, _ uint64) error {
				count++
				return nil
			})).To(Succeed())
			Expect(count).To(Equal(expectedCount))
			svcCount := 0
			Expect(store.IterateServiceConnections(func(_, _ *net.IP, _, _, _, _ uint64) error {
				svcCount++
				return nil
			})).To(Succeed())
			Expect(svcCount).To(Equal(expectedServiceCount))
		},

		Entry("single ipv4 connections", []conntrack.Con{
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[0], Dst: &ipV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[1], Dst: &ipV4Addresses[0], Proto: dummyProto}},
		}, []bool{false}, 1, 0),
		Entry("single (short) ipv4 connections", []conntrack.Con{
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[0], Dst: &shortIPV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[1], Dst: &shortIPV4Addresses[0], Proto: dummyProto}},
		}, []bool{false}, 1, 0),
		Entry("single ipv6 connections", []conntrack.Con{
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[0], Dst: &ipV6Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[1], Dst: &ipV6Addresses[0], Proto: dummyProto}},
		}, []bool{false}, 1, 0),
		Entry("multiple ipv4 connections", []conntrack.Con{
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[0], Dst: &ipV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[1], Dst: &ipV4Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[2], Dst: &ipV4Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[3], Dst: &ipV4Addresses[2], Proto: dummyProto}},
			{ID: &connID3, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[4], Dst: &ipV4Addresses[0], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[0], Dst: &ipV4Addresses[4], Proto: dummyProto}},
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[0], Dst: &ipV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[1], Dst: &ipV4Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[2], Dst: &ipV4Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[3], Dst: &ipV4Addresses[2], Proto: dummyProto}},
		}, []bool{false, false, false, false, false}, 3, 0),
		Entry("multiple (short) ipv4 connections", []conntrack.Con{
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[0], Dst: &shortIPV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[1], Dst: &shortIPV4Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[2], Dst: &shortIPV4Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[3], Dst: &shortIPV4Addresses[2], Proto: dummyProto}},
			{ID: &connID3, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[4], Dst: &shortIPV4Addresses[0], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[0], Dst: &shortIPV4Addresses[4], Proto: dummyProto}},
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[0], Dst: &shortIPV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[1], Dst: &shortIPV4Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[2], Dst: &shortIPV4Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[3], Dst: &shortIPV4Addresses[2], Proto: dummyProto}},
		}, []bool{false, false, false, false, false}, 3, 0),
		Entry("multiple ipv6 connections", []conntrack.Con{
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[0], Dst: &ipV6Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[1], Dst: &ipV6Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[2], Dst: &ipV6Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[3], Dst: &ipV6Addresses[2], Proto: dummyProto}},
			{ID: &connID3, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[4], Dst: &ipV6Addresses[0], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[0], Dst: &ipV6Addresses[4], Proto: dummyProto}},
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[0], Dst: &ipV6Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[1], Dst: &ipV6Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[2], Dst: &ipV6Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[3], Dst: &ipV6Addresses[2], Proto: dummyProto}},
		}, []bool{false, false, false, false, false}, 3, 0),
		Entry("multiple ipv4 connections with service connections and close operations", []conntrack.Con{
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[0], Dst: &ipV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[1], Dst: &ipV4Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[2], Dst: &ipV4Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[4], Dst: &ipV4Addresses[2], Proto: dummyProto}},
			{ID: &connID3, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[4], Dst: &ipV4Addresses[0], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[0], Dst: &ipV4Addresses[4], Proto: dummyProto}},
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[0], Dst: &ipV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[1], Dst: &ipV4Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &ipV4Addresses[2], Dst: &ipV4Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV4Addresses[4], Dst: &ipV4Addresses[2], Proto: dummyProto}},
		}, []bool{false, true, true, true, false}, 1, 1),
		Entry("multiple (short) ipv4 connections with service connections and close operations", []conntrack.Con{
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[0], Dst: &shortIPV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[1], Dst: &shortIPV4Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[2], Dst: &shortIPV4Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[4], Dst: &shortIPV4Addresses[2], Proto: dummyProto}},
			{ID: &connID3, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[4], Dst: &shortIPV4Addresses[0], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[0], Dst: &shortIPV4Addresses[4], Proto: dummyProto}},
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[0], Dst: &shortIPV4Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[1], Dst: &shortIPV4Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &shortIPV4Addresses[2], Dst: &shortIPV4Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &shortIPV4Addresses[4], Dst: &shortIPV4Addresses[2], Proto: dummyProto}},
		}, []bool{false, true, true, true, false}, 1, 1),
		Entry("multiple ipv6 connections with service connections and close operations", []conntrack.Con{
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[0], Dst: &ipV6Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[1], Dst: &ipV6Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[2], Dst: &ipV6Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[4], Dst: &ipV6Addresses[2], Proto: dummyProto}},
			{ID: &connID3, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[4], Dst: &ipV6Addresses[0], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[0], Dst: &ipV6Addresses[4], Proto: dummyProto}},
			{ID: &connID1, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[0], Dst: &ipV6Addresses[1], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[1], Dst: &ipV6Addresses[0], Proto: dummyProto}},
			{ID: &connID2, Origin: &conntrack.IPTuple{Src: &ipV6Addresses[2], Dst: &ipV6Addresses[3], Proto: dummyProto}, Reply: &conntrack.IPTuple{Src: &ipV6Addresses[4], Dst: &ipV6Addresses[2], Proto: dummyProto}},
		}, []bool{false, true, true, true, false}, 1, 1),
	)
})
