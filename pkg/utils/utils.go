// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"net"
	"net/netip"
	"time"

	ct "github.com/florianl/go-conntrack"
	"github.com/sirupsen/logrus"
)

const (
	tcpProtocolNumber = 6
	udpProtocolNumber = 17
)

func GetCounterBytesIfAvailable(counter *ct.Counter) uint64 {
	if counter != nil {
		return *counter.Bytes
	}
	return 0
}

func GetCounterPacketsIfAvailable(counter *ct.Counter) uint64 {
	if counter != nil {
		return *counter.Packets
	}
	return 0
}

func GetStartTimeIfAvailable(timestamp *ct.Timestamp) *time.Time {
	if timestamp != nil {
		return timestamp.Start
	}
	return nil
}

func GetStopTimeIfAvailable(timestamp *ct.Timestamp) *time.Time {
	if timestamp != nil {
		return timestamp.Stop
	}
	return nil
}

func getIPProtocol(number uint8) string {
	switch number {
	case tcpProtocolNumber:
		return "TCP"
	case udpProtocolNumber:
		return "UDP"
	default:
		return fmt.Sprintf("proto %d", number)
	}
}

func GetTime(time *time.Time, c *ct.Con) *time.Time {
	if time != nil {
		return time
	}
	t := time
	if startTime := GetStartTimeIfAvailable(c.Timestamp); startTime != nil {
		t = startTime
	}
	if stopTime := GetStopTimeIfAvailable(c.Timestamp); stopTime != nil {
		t = stopTime
	}
	return t
}

func GetSourcePortIfAvailable(proto *ct.ProtoTuple) uint16 {
	if isTCPOrUDP(proto) {
		return *proto.SrcPort
	}
	return 0
}

func GetDestinationPortIfAvailable(proto *ct.ProtoTuple) uint16 {
	if isTCPOrUDP(proto) {
		return *proto.DstPort
	}
	return 0
}

func TraceConnection(enabled bool, log *logrus.Entry, c *ct.Con, source string) {
	if enabled {
		// Not all protocols have a notion of ports, handle tcp/udp specifically as the major cases
		switch {
		case !isTCPOrUDP(c.Origin.Proto):
			log.Infof("Received connection via %s: %s->%s (%s->%s), %s, bytes sent/received %d/%d, packets sent/received %d/%d, id %d, status %d, time %s->%s", source,
				c.Origin.Src, c.Origin.Dst, c.Reply.Dst, c.Reply.Src, getIPProtocol(*c.Origin.Proto.Number),
				GetCounterBytesIfAvailable(c.CounterOrigin), GetCounterBytesIfAvailable(c.CounterReply),
				GetCounterPacketsIfAvailable(c.CounterOrigin), GetCounterPacketsIfAvailable(c.CounterReply),
				*c.ID, c.Status, GetStartTimeIfAvailable(c.Timestamp), GetStopTimeIfAvailable(c.Timestamp))
		case c.Origin.Dst.Equal(*c.Reply.Src) && GetDestinationPortIfAvailable(c.Origin.Proto) == GetSourcePortIfAvailable(c.Reply.Proto) &&
			c.Origin.Src.Equal(*c.Reply.Dst) && GetSourcePortIfAvailable(c.Origin.Proto) == GetDestinationPortIfAvailable(c.Reply.Proto):
			// No network address translation (NAT) => no need to print reply addresses as they are the same
			log.Infof("Received connection via %s: %s:%d->%s:%d, %s, bytes sent/received %d/%d, packets sent/received %d/%d, id %d, status %d, time %s->%s", source,
				c.Origin.Src, GetSourcePortIfAvailable(c.Origin.Proto), c.Origin.Dst, GetDestinationPortIfAvailable(c.Origin.Proto),
				getIPProtocol(*c.Origin.Proto.Number),
				GetCounterBytesIfAvailable(c.CounterOrigin), GetCounterBytesIfAvailable(c.CounterReply),
				GetCounterPacketsIfAvailable(c.CounterOrigin), GetCounterPacketsIfAvailable(c.CounterReply),
				*c.ID, c.Status, GetStartTimeIfAvailable(c.Timestamp), GetStopTimeIfAvailable(c.Timestamp))
		default:
			log.Infof("Received connection via %s: %s:%d->%s:%d (%s:%d->%s:%d), %s, bytes sent/received %d/%d, packets sent/received %d/%d, id %d, status %d, time %s->%s", source,
				c.Origin.Src, GetSourcePortIfAvailable(c.Origin.Proto), c.Origin.Dst, GetDestinationPortIfAvailable(c.Origin.Proto),
				c.Reply.Dst, GetDestinationPortIfAvailable(c.Reply.Proto), c.Reply.Src, GetSourcePortIfAvailable(c.Reply.Proto),
				getIPProtocol(*c.Origin.Proto.Number),
				GetCounterBytesIfAvailable(c.CounterOrigin), GetCounterBytesIfAvailable(c.CounterReply),
				GetCounterPacketsIfAvailable(c.CounterOrigin), GetCounterPacketsIfAvailable(c.CounterReply),
				*c.ID, c.Status, GetStartTimeIfAvailable(c.Timestamp), GetStopTimeIfAvailable(c.Timestamp))
		}
	}
}

func ConvertIP(ip *net.IP) (netip.Addr, bool) {
	if ipv4 := ip.To4(); ipv4 != nil {
		return netip.AddrFromSlice(ipv4)
	}
	return netip.AddrFromSlice(*ip)
}

func isTCPOrUDP(proto *ct.ProtoTuple) bool {
	return *proto.Number == tcpProtocolNumber || *proto.Number == udpProtocolNumber
}
