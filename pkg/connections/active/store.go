// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package active

import (
	"net"
	"sync"
	"time"

	"github.com/gardener/network-traffic-gauger/pkg/connections/closed"
	"github.com/gardener/network-traffic-gauger/pkg/connections/lookup"
	"github.com/gardener/network-traffic-gauger/pkg/utils"

	ct "github.com/florianl/go-conntrack"
	"github.com/sirupsen/logrus"
)

type Store interface {
	StartStorageWorker()
	StopStorageWorker()
	HandleConnection(c *ct.Con, traceSource string, closed bool, time *time.Time)
	IterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error) error
	IterateServiceConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error) error
}

type store struct {
	openConnections        map[uint32]*connectionData
	openConnectionsLock    sync.RWMutex
	lookupTable            lookup.ActiveConnectionPairs
	storageChannel         chan connectionData
	closedConnections      closed.Store
	traceClosedConnections bool
	traceConnectionChange  bool
	log                    *logrus.Entry
}

type connectionData struct {
	src             net.IP
	srcPort         uint16
	dst             net.IP
	dstPort         uint16
	svcDst          net.IP
	svcDstPort      uint16
	sentBytes       uint64
	receivedBytes   uint64
	sentPackets     uint64
	receivedPackets uint64
	lastChange      time.Time
}

func NewStore(lookupTable lookup.ActiveConnectionPairs, closedConnections closed.Store, channelBufferSize int, traceClosedConnections bool, traceConnectionChange bool) Store {
	return &store{
		openConnections:        map[uint32]*connectionData{},
		lookupTable:            lookupTable,
		storageChannel:         make(chan connectionData, channelBufferSize),
		closedConnections:      closedConnections,
		traceClosedConnections: traceClosedConnections,
		traceConnectionChange:  traceConnectionChange,
		log:                    logrus.WithField("component", "active-connections-store"),
	}
}

func (ms *store) StartStorageWorker() {
	// Decouple writing of flows from event processing so that i/o does not block the main event loop
	go func() {
		for cd := range ms.storageChannel {
			if cd.isEmpty() {
				ms.log.Infof("Empty flow received, exiting storage channel...")
				break
			}
			if ms.traceClosedConnections {
				ms.log.Infof("About to store flow: %s:%d->%s:%d (%s:%d), sent/received %d/%d (%d/%d)",
					cd.src, cd.srcPort, cd.dst, cd.dstPort, cd.svcDst, cd.svcDstPort, cd.sentBytes, cd.receivedBytes, cd.sentPackets, cd.receivedPackets)
			}
			if err := ms.closedConnections.StoreFlow(&cd.src, &cd.dst, &cd.svcDst, cd.sentBytes, cd.receivedBytes, cd.sentPackets, cd.receivedPackets, cd.lastChange); err != nil {
				ms.log.Errorf("Failed to store flow: %s:%d->%s:%d (%s:%d), sent/received %d/%d (%d/%d), reason: %v",
					cd.src, cd.srcPort, cd.dst, cd.dstPort, cd.svcDst, cd.svcDstPort, cd.sentBytes, cd.receivedBytes, cd.sentPackets, cd.receivedPackets, err)
			}
			ms.lookupTable.Remove(cd.src, cd.dst, cd.svcDst)
		}
	}()
}

func (ms *store) StopStorageWorker() {
	close(ms.storageChannel)
}

func (ms *store) HandleConnection(c *ct.Con, traceSource string, closed bool, time *time.Time) {
	ms.openConnectionsLock.Lock()
	defer ms.openConnectionsLock.Unlock()
	cd, exists := ms.openConnections[*c.ID]
	switch {
	case !exists:
		cd = newConnectionData(c, time)
		if !closed {
			ms.openConnections[*c.ID] = cd
		}
		ms.lookupTable.Add(cd.src, cd.dst, cd.svcDst)
		if ms.traceConnectionChange {
			ms.log.Infof("Found new connection via %s: %s:%d->%s:%d (%s:%d)", traceSource,
				cd.src, cd.srcPort, cd.dst, cd.dstPort, cd.svcDst, cd.svcDstPort)
		}
	case !cd.equalConnection(c):
		ms.log.Warnf("Missed close event of connection %s:%d->%s:%d (%s:%d), connection id reused for %s:%d->%s:%d (%s:%d)",
			cd.src, cd.srcPort, cd.dst, cd.dstPort, cd.svcDst, cd.svcDstPort,
			c.Origin.Src, utils.GetSourcePortIfAvailable(c.Origin.Proto),
			c.Reply.Src, utils.GetSourcePortIfAvailable(c.Reply.Proto),
			c.Origin.Dst, utils.GetDestinationPortIfAvailable(c.Origin.Proto))
		if ms.traceConnectionChange {
			ms.log.Infof("Indirectly found closed connection via %s: %s:%d->%s:%d (%s:%d)", traceSource,
				cd.src, cd.srcPort, cd.dst, cd.dstPort, cd.svcDst, cd.svcDstPort)
		}
		ms.storageChannel <- *cd
		cd.reset(c, time)
		ms.lookupTable.Add(cd.src, cd.dst, cd.svcDst)
		if ms.traceConnectionChange {
			ms.log.Infof("Indirectly found new connection via %s: %s:%d->%s:%d (%s:%d)", traceSource,
				cd.src, cd.srcPort, cd.dst, cd.dstPort, cd.svcDst, cd.svcDstPort)
		}
	default:
		cd.updateCounters(c, time)
		if ms.traceConnectionChange {
			ms.log.Infof("Updated existing connection via %s: %s:%d->%s:%d (%s:%d)", traceSource,
				cd.src, cd.srcPort, cd.dst, cd.dstPort, cd.svcDst, cd.svcDstPort)
		}
	}
	if closed {
		if exists {
			delete(ms.openConnections, *c.ID)
		}
		ms.storageChannel <- *cd
		if ms.traceConnectionChange {
			ms.log.Infof("Found closed connection via %s: %s:%d->%s:%d (%s:%d)", traceSource,
				cd.src, cd.srcPort, cd.dst, cd.dstPort, cd.svcDst, cd.svcDstPort)
		}
	}
}

func (ms *store) IterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error) error {
	return ms.iterateConnections(callback,
		func(_ *connectionData) bool { return true },
		func(data *connectionData) *net.IP { return &data.dst })
}

func (ms *store) IterateServiceConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error) error {
	return ms.iterateConnections(callback,
		func(data *connectionData) bool { return !data.dst.Equal(data.svcDst) },
		func(data *connectionData) *net.IP { return &data.svcDst })
}

func (ms *store) iterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error, isRelevant func(data *connectionData) bool, destination func(data *connectionData) *net.IP) error {
	ms.openConnectionsLock.RLock()
	defer ms.openConnectionsLock.RUnlock()
	for _, cd := range ms.openConnections {
		if isRelevant(cd) {
			if err := callback(&cd.src, destination(cd), cd.sentBytes, cd.receivedBytes, cd.sentPackets, cd.receivedPackets); err != nil {
				return err
			}
		}
	}
	return nil
}

func newConnectionData(c *ct.Con, time *time.Time) *connectionData {
	cd := &connectionData{}
	cd.reset(c, time)
	return cd
}

func (cd *connectionData) equalConnection(c *ct.Con) bool {
	return cd.src.Equal(*c.Origin.Src) && cd.srcPort == utils.GetSourcePortIfAvailable(c.Origin.Proto) &&
		cd.dst.Equal(*c.Reply.Src) && cd.dstPort == utils.GetSourcePortIfAvailable(c.Reply.Proto) &&
		cd.svcDst.Equal(*c.Origin.Dst) && cd.svcDstPort == utils.GetDestinationPortIfAvailable(c.Origin.Proto)
}

func (cd *connectionData) updateCounters(c *ct.Con, time *time.Time) {
	cd.sentBytes = utils.GetCounterBytesIfAvailable(c.CounterOrigin)
	cd.receivedBytes = utils.GetCounterBytesIfAvailable(c.CounterReply)
	cd.sentPackets = utils.GetCounterPacketsIfAvailable(c.CounterOrigin)
	cd.receivedPackets = utils.GetCounterPacketsIfAvailable(c.CounterReply)
	if time != nil {
		cd.lastChange = *time
	}
}

func (cd *connectionData) reset(c *ct.Con, time *time.Time) {
	cd.src = *c.Origin.Src
	cd.srcPort = utils.GetSourcePortIfAvailable(c.Origin.Proto)
	cd.dst = *c.Reply.Src
	cd.dstPort = utils.GetSourcePortIfAvailable(c.Reply.Proto)
	cd.svcDst = *c.Origin.Dst
	cd.svcDstPort = utils.GetDestinationPortIfAvailable(c.Origin.Proto)
	cd.sentBytes = utils.GetCounterBytesIfAvailable(c.CounterOrigin)
	cd.receivedBytes = utils.GetCounterBytesIfAvailable(c.CounterReply)
	cd.sentPackets = utils.GetCounterPacketsIfAvailable(c.CounterOrigin)
	cd.receivedPackets = utils.GetCounterPacketsIfAvailable(c.CounterReply)
	if time != nil {
		cd.lastChange = *time
	}
}

func (cd *connectionData) isEmpty() bool {
	emptyCD := connectionData{}
	return cd.src == nil && cd.srcPort == emptyCD.srcPort &&
		cd.dst == nil && cd.dstPort == emptyCD.dstPort &&
		cd.svcDst == nil && cd.svcDstPort == emptyCD.svcDstPort &&
		cd.sentBytes == emptyCD.sentBytes && cd.receivedBytes == emptyCD.receivedBytes &&
		cd.sentPackets == emptyCD.sentPackets && cd.receivedPackets == emptyCD.receivedPackets &&
		cd.lastChange.Equal(emptyCD.lastChange)
}
