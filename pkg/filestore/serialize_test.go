// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package filestore

import (
	"bytes"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("serialization/deserialization", func() {
	Describe("serialize/deserialize ip addresses", func() {
		var (
			ipV4Addresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0"), net.ParseIP("255.255.255.255"), net.ParseIP("192.168.123.45"), net.ParseIP("10.11.12.13")}
			ipV6Addresses = []net.IP{net.ParseIP("::1"), net.ParseIP("::"), net.ParseIP("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"), net.ParseIP("2001:db8:1234:5678:abcd:ef01:9876:5432"), net.ParseIP("fc00::7654")}
			longByteArray = net.IP(make([]byte, 256))
		)

		DescribeTable("should serialize/deserialize ip addresses",
			func(ip *net.IP, expectError bool) {
				data, err := serializeIP(ip)

				if expectError {
					Expect(err).NotTo(BeNil())
					Expect(data).To(BeNil())
					return
				}
				Expect(err).To(BeNil())
				Expect(data).NotTo(BeNil())

				readIp, n, err := deserializeIP(bytes.NewReader(data))

				Expect(err).To(BeNil())
				Expect(n).To(Equal(len(data)))
				Expect(ip.Equal(*readIp)).To(BeTrue())
			},

			Entry("[256]byte{}", &longByteArray, true),
			Entry(ipV4Addresses[0].String(), &ipV4Addresses[0], false),
			Entry(ipV4Addresses[1].String(), &ipV4Addresses[1], false),
			Entry(ipV4Addresses[2].String(), &ipV4Addresses[2], false),
			Entry(ipV4Addresses[3].String(), &ipV4Addresses[3], false),
			Entry(ipV4Addresses[4].String(), &ipV4Addresses[4], false),
			Entry(ipV6Addresses[0].String(), &ipV6Addresses[0], false),
			Entry(ipV6Addresses[1].String(), &ipV6Addresses[1], false),
			Entry(ipV6Addresses[2].String(), &ipV6Addresses[2], false),
			Entry(ipV6Addresses[3].String(), &ipV6Addresses[3], false),
			Entry(ipV6Addresses[4].String(), &ipV6Addresses[4], false),
		)
	})

	Describe("serialize/deserialize data", func() {
		DescribeTable("should serialize/deserialize data",
			func(sentBytes, receivedBytes, sentPackets, receivedPackets, flowCount uint64, expectError bool) {
				data, err := serializeData(sentBytes, receivedBytes, sentPackets, receivedPackets, flowCount)

				if expectError {
					Expect(err).NotTo(BeNil())
					Expect(data).To(BeNil())
					return
				}
				Expect(err).To(BeNil())
				Expect(data).NotTo(BeNil())
				Expect(len(data)).To(Equal(5 * 8))

				readSentBytes, readReceivedBytes, readSentPackets, readReceivedPackets, readFlowCount, n, err := deserializeData(bytes.NewReader(data))

				Expect(err).To(BeNil())
				Expect(n).To(Equal(len(data)))
				Expect(readSentBytes).To(Equal(sentBytes))
				Expect(readReceivedBytes).To(Equal(receivedBytes))
				Expect(readSentPackets).To(Equal(sentPackets))
				Expect(readReceivedPackets).To(Equal(receivedPackets))
				Expect(readFlowCount).To(Equal(flowCount))
			},

			Entry("test data 1", uint64(123456789), uint64(987654321), uint64(12), uint64(98), uint64(5463738210), false),
			Entry("test data 2", uint64(1234567890), uint64(9876543210), uint64(1), uint64(0), uint64(1463738250), false),
		)
	})
})
