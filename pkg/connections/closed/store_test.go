// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package closed

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/gardener/network-traffic-gauger/pkg/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type flow struct {
	src             *net.IP
	dst             *net.IP
	svcDst          *net.IP
	sentBytes       uint64
	receivedBytes   uint64
	sentPackets     uint64
	receivedPackets uint64
}

var _ = Describe("file store test", func() {
	var (
		ipV4Addresses      = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0"), net.ParseIP("255.255.255.255"), net.ParseIP("192.168.123.45"), net.ParseIP("10.11.12.13")}
		shortIpV4Addresses = []net.IP{net.ParseIP("127.0.0.1").To4(), net.ParseIP("0.0.0.0").To4(), net.ParseIP("255.255.255.255").To4(), net.ParseIP("192.168.123.45").To4(), net.ParseIP("10.11.12.13").To4()}
		ipV6Addresses      = []net.IP{net.ParseIP("::1"), net.ParseIP("::"), net.ParseIP("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"), net.ParseIP("2001:db8:1234:5678:abcd:ef01:9876:5432"), net.ParseIP("fc00::7654")}

		store Store
	)

	BeforeEach(func() {
		store = NewStore()
	})

	DescribeTable("should store and reload flow data",
		func(flows []flow, expectError bool) {
			By("empty store")
			entries := 0
			Expect(store.IterateConnections(func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error {
				entries++
				return nil
			})).To(BeNil())
			Expect(entries).To(BeZero())
			Expect(store.IterateServiceConnections(func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error {
				entries++
				return nil
			})).To(BeNil())
			Expect(entries).To(BeZero())

			By("store flows")
			for n, f := range flows {
				By(fmt.Sprintf("storing flow %d", n))
				Expect(store.StoreFlow(f.src, f.dst, f.svcDst, f.sentBytes, f.receivedBytes, f.sentPackets, f.receivedPackets)).To(BeNil())
			}

			By("get intermediate copy of flow data")
			ordinaryConnections := map[netip.Addr]map[netip.Addr]*flow{}
			ordinaryConnectionCounts := map[netip.Addr]map[netip.Addr]*uint64{}
			serviceConnections := map[netip.Addr]map[netip.Addr]*flow{}
			serviceConnectionCounts := map[netip.Addr]map[netip.Addr]*uint64{}
			Expect(store.IterateConnections(storeConnectionClosure(ordinaryConnections, ordinaryConnectionCounts))).To(BeNil())
			Expect(store.IterateServiceConnections(storeConnectionClosure(serviceConnections, serviceConnectionCounts))).To(BeNil())

			By("calculate flows")
			calculatedOrdinaryConnections := map[netip.Addr]map[netip.Addr]*flow{}
			calculatedOrdinaryConnectionCounts := map[netip.Addr]map[netip.Addr]*uint64{}
			calculatedServiceConnections := map[netip.Addr]map[netip.Addr]*flow{}
			calculatedServiceConnectionCounts := map[netip.Addr]map[netip.Addr]*uint64{}
			for n, f := range flows {
				By(fmt.Sprintf("calculating flow %d", n))
				srcKey, ok := utils.ConvertIP(f.src)
				Expect(ok).To(BeTrue())
				dstMap, exists := calculatedOrdinaryConnections[srcKey]
				if !exists {
					dstMap = map[netip.Addr]*flow{}
					calculatedOrdinaryConnections[srcKey] = dstMap
				}
				dstCountsMap, exists := calculatedOrdinaryConnectionCounts[srcKey]
				if !exists {
					dstCountsMap = map[netip.Addr]*uint64{}
					calculatedOrdinaryConnectionCounts[srcKey] = dstCountsMap
				}
				dstKey, ok := utils.ConvertIP(f.dst)
				Expect(ok).To(BeTrue())
				calculatedFlow, exists := dstMap[dstKey]
				if !exists {
					calculatedFlow = &flow{
						src:    f.src,
						dst:    f.dst,
						svcDst: f.dst,
					}
					dstMap[dstKey] = calculatedFlow
				}
				calculatedFlow.receivedBytes += f.receivedBytes
				calculatedFlow.receivedPackets += f.receivedPackets
				calculatedFlow.sentBytes += f.sentBytes
				calculatedFlow.sentPackets += f.sentPackets
				calculatedCount, exists := dstCountsMap[dstKey]
				if !exists {
					var count uint64 = 0
					calculatedCount = &count
					dstCountsMap[dstKey] = calculatedCount
				}
				*calculatedCount += 1
				if !f.dst.Equal(*f.svcDst) {
					svcDstMap, exists := calculatedServiceConnections[srcKey]
					if !exists {
						svcDstMap = map[netip.Addr]*flow{}
						calculatedServiceConnections[srcKey] = svcDstMap
					}
					svcDstCountsMap, exists := calculatedServiceConnectionCounts[srcKey]
					if !exists {
						svcDstCountsMap = map[netip.Addr]*uint64{}
						calculatedServiceConnectionCounts[srcKey] = svcDstCountsMap
					}
					svcDstKey, ok := utils.ConvertIP(f.svcDst)
					Expect(ok).To(BeTrue())
					calculatedSvcFlow, exists := svcDstMap[svcDstKey]
					if !exists {
						calculatedSvcFlow = &flow{
							src:    f.src,
							dst:    f.svcDst,
							svcDst: f.svcDst,
						}
						svcDstMap[svcDstKey] = calculatedSvcFlow
					}
					calculatedSvcFlow.receivedBytes += f.receivedBytes
					calculatedSvcFlow.receivedPackets += f.receivedPackets
					calculatedSvcFlow.sentBytes += f.sentBytes
					calculatedSvcFlow.sentPackets += f.sentPackets
					calculatedSvcCount, exists := svcDstCountsMap[svcDstKey]
					if !exists {
						var count uint64 = 0
						calculatedSvcCount = &count
						svcDstCountsMap[svcDstKey] = calculatedSvcCount
					}
					*calculatedSvcCount += 1
				}
			}

			By("compare data")
			compareConnections(ordinaryConnections, ordinaryConnectionCounts, calculatedOrdinaryConnections, calculatedOrdinaryConnectionCounts)
			compareConnections(serviceConnections, serviceConnectionCounts, calculatedServiceConnections, calculatedServiceConnectionCounts)
		},

		Entry("no flow", []flow{}, false),
		Entry("single ipv4 flow", []flow{{&ipV4Addresses[0], &ipV4Addresses[1], &ipV4Addresses[2], uint64(1), uint64(2), uint64(3), uint64(4)}}, false),
		Entry("single (short) ipv4 flow", []flow{{&shortIpV4Addresses[0], &shortIpV4Addresses[1], &shortIpV4Addresses[2], uint64(1), uint64(2), uint64(3), uint64(4)}}, false),
		Entry("single ipv6 flow", []flow{{&ipV6Addresses[0], &ipV6Addresses[1], &ipV6Addresses[2], uint64(1234567890), uint64(987654321), uint64(1029384756), uint64(918273645)}}, false),
		Entry("several separate ipv4 flows", []flow{
			{&ipV4Addresses[0], &ipV4Addresses[1], &ipV4Addresses[2], uint64(1), uint64(2), uint64(3), uint64(4)},
			{&ipV4Addresses[1], &ipV4Addresses[2], &ipV4Addresses[3], uint64(10), uint64(20), uint64(30), uint64(40)},
			{&ipV4Addresses[2], &ipV4Addresses[3], &ipV4Addresses[4], uint64(100), uint64(200), uint64(300), uint64(400)},
			{&ipV4Addresses[3], &ipV4Addresses[4], &ipV4Addresses[0], uint64(1000), uint64(2000), uint64(3000), uint64(4000)},
			{&ipV4Addresses[4], &ipV4Addresses[0], &ipV4Addresses[1], uint64(10000), uint64(20000), uint64(30000), uint64(40000)},
		}, false),
		Entry("several separate (short) ipv4 flows", []flow{
			{&shortIpV4Addresses[0], &shortIpV4Addresses[1], &shortIpV4Addresses[2], uint64(1), uint64(2), uint64(3), uint64(4)},
			{&shortIpV4Addresses[1], &shortIpV4Addresses[2], &shortIpV4Addresses[3], uint64(10), uint64(20), uint64(30), uint64(40)},
			{&shortIpV4Addresses[2], &shortIpV4Addresses[3], &shortIpV4Addresses[4], uint64(100), uint64(200), uint64(300), uint64(400)},
			{&shortIpV4Addresses[3], &shortIpV4Addresses[4], &shortIpV4Addresses[0], uint64(1000), uint64(2000), uint64(3000), uint64(4000)},
			{&shortIpV4Addresses[4], &shortIpV4Addresses[0], &shortIpV4Addresses[1], uint64(10000), uint64(20000), uint64(30000), uint64(40000)},
		}, false),
		Entry("several separate ipv6 flows", []flow{
			{&ipV6Addresses[0], &ipV6Addresses[1], &ipV6Addresses[2], uint64(1), uint64(2), uint64(3), uint64(4)},
			{&ipV6Addresses[1], &ipV6Addresses[2], &ipV6Addresses[3], uint64(10), uint64(20), uint64(30), uint64(40)},
			{&ipV6Addresses[2], &ipV6Addresses[3], &ipV6Addresses[4], uint64(100), uint64(200), uint64(300), uint64(400)},
			{&ipV6Addresses[3], &ipV6Addresses[4], &ipV6Addresses[0], uint64(1000), uint64(2000), uint64(3000), uint64(4000)},
			{&ipV6Addresses[4], &ipV6Addresses[0], &ipV6Addresses[1], uint64(10000), uint64(20000), uint64(30000), uint64(40000)},
		}, false),
		Entry("several repeated ipv4 flows", []flow{
			{&ipV4Addresses[0], &ipV4Addresses[1], &ipV4Addresses[2], uint64(1), uint64(2), uint64(3), uint64(4)},
			{&ipV4Addresses[0], &ipV4Addresses[1], &ipV4Addresses[2], uint64(11), uint64(22), uint64(33), uint64(44)},
			{&ipV4Addresses[0], &ipV4Addresses[1], &ipV4Addresses[2], uint64(111), uint64(222), uint64(333), uint64(444)},
			{&ipV4Addresses[0], &ipV4Addresses[3], &ipV4Addresses[4], uint64(12), uint64(23), uint64(34), uint64(45)},
			{&ipV4Addresses[0], &ipV4Addresses[3], &ipV4Addresses[4], uint64(123), uint64(234), uint64(345), uint64(456)},
			{&ipV4Addresses[1], &ipV4Addresses[2], &ipV4Addresses[3], uint64(101), uint64(201), uint64(301), uint64(401)},
			{&ipV4Addresses[1], &ipV4Addresses[3], &ipV4Addresses[3], uint64(102), uint64(202), uint64(302), uint64(402)},
			{&ipV4Addresses[1], &ipV4Addresses[2], &ipV4Addresses[3], uint64(103), uint64(203), uint64(303), uint64(403)},
			{&ipV4Addresses[1], &ipV4Addresses[3], &ipV4Addresses[3], uint64(104), uint64(204), uint64(304), uint64(404)},
			{&ipV4Addresses[1], &ipV4Addresses[2], &ipV4Addresses[3], uint64(105), uint64(205), uint64(305), uint64(405)},
		}, false),
		Entry("several repeated (short) ipv4 flows", []flow{
			{&shortIpV4Addresses[0], &shortIpV4Addresses[1], &shortIpV4Addresses[2], uint64(1), uint64(2), uint64(3), uint64(4)},
			{&shortIpV4Addresses[0], &shortIpV4Addresses[1], &shortIpV4Addresses[2], uint64(11), uint64(22), uint64(33), uint64(44)},
			{&shortIpV4Addresses[0], &shortIpV4Addresses[1], &shortIpV4Addresses[2], uint64(111), uint64(222), uint64(333), uint64(444)},
			{&shortIpV4Addresses[0], &shortIpV4Addresses[3], &shortIpV4Addresses[4], uint64(12), uint64(23), uint64(34), uint64(45)},
			{&shortIpV4Addresses[0], &shortIpV4Addresses[3], &shortIpV4Addresses[4], uint64(123), uint64(234), uint64(345), uint64(456)},
			{&shortIpV4Addresses[1], &shortIpV4Addresses[2], &shortIpV4Addresses[3], uint64(101), uint64(201), uint64(301), uint64(401)},
			{&shortIpV4Addresses[1], &shortIpV4Addresses[3], &shortIpV4Addresses[3], uint64(102), uint64(202), uint64(302), uint64(402)},
			{&shortIpV4Addresses[1], &shortIpV4Addresses[2], &shortIpV4Addresses[3], uint64(103), uint64(203), uint64(303), uint64(403)},
			{&shortIpV4Addresses[1], &shortIpV4Addresses[3], &shortIpV4Addresses[3], uint64(104), uint64(204), uint64(304), uint64(404)},
			{&shortIpV4Addresses[1], &shortIpV4Addresses[2], &shortIpV4Addresses[3], uint64(105), uint64(205), uint64(305), uint64(405)},
		}, false),
		Entry("several repeated ipv6 flows", []flow{
			{&ipV6Addresses[0], &ipV6Addresses[1], &ipV6Addresses[2], uint64(1), uint64(2), uint64(3), uint64(4)},
			{&ipV6Addresses[0], &ipV6Addresses[1], &ipV6Addresses[2], uint64(11), uint64(22), uint64(33), uint64(44)},
			{&ipV6Addresses[0], &ipV6Addresses[1], &ipV6Addresses[2], uint64(111), uint64(222), uint64(333), uint64(444)},
			{&ipV6Addresses[0], &ipV6Addresses[3], &ipV6Addresses[4], uint64(12), uint64(23), uint64(34), uint64(45)},
			{&ipV6Addresses[0], &ipV6Addresses[3], &ipV6Addresses[4], uint64(123), uint64(234), uint64(345), uint64(456)},
			{&ipV6Addresses[1], &ipV6Addresses[2], &ipV6Addresses[3], uint64(101), uint64(201), uint64(301), uint64(401)},
			{&ipV6Addresses[1], &ipV6Addresses[3], &ipV6Addresses[3], uint64(102), uint64(202), uint64(302), uint64(402)},
			{&ipV6Addresses[1], &ipV6Addresses[2], &ipV6Addresses[3], uint64(103), uint64(203), uint64(303), uint64(403)},
			{&ipV6Addresses[1], &ipV6Addresses[3], &ipV6Addresses[3], uint64(104), uint64(204), uint64(304), uint64(404)},
			{&ipV6Addresses[1], &ipV6Addresses[2], &ipV6Addresses[3], uint64(105), uint64(205), uint64(305), uint64(405)},
		}, false),
	)
})

func storeConnectionClosure(connections map[netip.Addr]map[netip.Addr]*flow, counts map[netip.Addr]map[netip.Addr]*uint64) func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error {
	return func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error {
		srcIp, ok := netip.AddrFromSlice(*src)
		if !ok {
			return fmt.Errorf("could not convert source ip addres: %s", src.String())
		}
		dstIp, ok := netip.AddrFromSlice(*dst)
		if !ok {
			return fmt.Errorf("could not convert destination ip addres: %s", dst.String())
		}
		flows, ok := connections[srcIp]
		if !ok {
			flows = map[netip.Addr]*flow{}
			connections[srcIp] = flows
		}
		flowCounts, ok := counts[srcIp]
		if !ok {
			flowCounts = map[netip.Addr]*uint64{}
			counts[srcIp] = flowCounts
		}
		data, ok := flows[dstIp]
		if !ok {
			data = &flow{
				src:    src,
				dst:    dst,
				svcDst: dst,
			}
			flows[dstIp] = data
		}
		data.sentBytes = sentBytes
		data.receivedBytes = receivedBytes
		data.sentPackets = sentPackets
		data.receivedPackets = receivedPackets
		c, ok := flowCounts[dstIp]
		if !ok {
			var value uint64 = 0
			c = &value
			flowCounts[dstIp] = c
		}
		*c = count
		return nil
	}
}

func compareConnections(connections1 map[netip.Addr]map[netip.Addr]*flow, counts1 map[netip.Addr]map[netip.Addr]*uint64, connections2 map[netip.Addr]map[netip.Addr]*flow, counts2 map[netip.Addr]map[netip.Addr]*uint64) {
	Expect(len(connections1)).To(Equal(len(counts1)))
	Expect(len(connections1)).To(Equal(len(connections2)))
	Expect(len(counts1)).To(Equal(len(counts2)))
	for src, flows1 := range connections1 {
		flowCounts1, exists := counts1[src]
		Expect(exists).To(BeTrue(), fmt.Sprintf("source ip %s missing from source counts", src.String()))
		flows2, exists := connections2[src]
		Expect(exists).To(BeTrue(), fmt.Sprintf("source ip %s missing from target connections", src.String()))
		flowCounts2, exists := counts2[src]
		Expect(exists).To(BeTrue(), fmt.Sprintf("source ip %s missing from target counts", src.String()))
		Expect(len(flows1)).To(Equal(len(flowCounts1)))
		Expect(len(flows1)).To(Equal(len(flows2)))
		Expect(len(flowCounts1)).To(Equal(len(flowCounts2)))
		for dst, f1 := range flows1 {
			c1, exists := flowCounts1[dst]
			Expect(exists).To(BeTrue(), fmt.Sprintf("destination ip %s missing from destination counts", dst.String()))
			f2, exists := flows2[dst]
			Expect(exists).To(BeTrue(), fmt.Sprintf("destination ip %s missing from target destination flows", dst.String()))
			c2, exists := flowCounts2[dst]
			Expect(exists).To(BeTrue(), fmt.Sprintf("destination ip %s missing from target destination counts", dst.String()))
			Expect(f1.src.Equal(*f2.src)).To(BeTrue(), "source not equal: %s vs. %s", f1.src.String(), f2.src.String())
			Expect(f1.dst.Equal(*f2.dst)).To(BeTrue(), "destination not equal: %s vs. %s", f1.dst.String(), f2.dst.String())
			Expect(f1.svcDst.Equal(*f2.svcDst)).To(BeTrue(), "service destination not equal: %s vs. %s", f1.svcDst.String(), f2.svcDst.String())
			Expect(f1.sentBytes).To(Equal(f2.sentBytes))
			Expect(f1.receivedBytes).To(Equal(f2.receivedBytes))
			Expect(f1.sentPackets).To(Equal(f2.sentPackets))
			Expect(f1.receivedPackets).To(Equal(f2.receivedPackets))
			Expect(*c1).To(Equal(*c2))
		}
	}
}
