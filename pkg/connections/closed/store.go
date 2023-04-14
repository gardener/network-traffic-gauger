// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package closed

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/gardener/network-traffic-gauger/pkg/connections/lookup"
	"github.com/gardener/network-traffic-gauger/pkg/utils"
	"github.com/sirupsen/logrus"
)

type Store interface {
	StartCleanupWorker()
	StopCleanupWorker()
	StoreFlow(src, dst, svcDst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64, t time.Time) error
	IterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error
	IterateServiceConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error
}

type store struct {
	dataPerSource       map[netip.Addr]*outgoingConnections
	dataPerSourceLock   sync.Mutex
	cleanupEnabled      bool
	cleanupReportedOnly bool
	cleanupPeriod       time.Duration
	cleanupTicker       *time.Ticker
	traceCleanups       bool
	lookupTable         lookup.ActiveConnectionPairs
	log                 *logrus.Entry
}

type outgoingConnections struct {
	realDestinations    map[netip.Addr]*connectionData
	serviceDestinations map[netip.Addr]*connectionData
}

type connectionData struct {
	sentBytes       uint64
	receivedBytes   uint64
	sentPackets     uint64
	receivedPackets uint64
	flowCount       uint64
	lastUpdate      time.Time
	reported        bool
}

func NewStore(lookupTable lookup.ActiveConnectionPairs, cleanupEnabled bool, cleanupReportedOnly bool, cleanupPeriod time.Duration, traceCleanups bool) Store {
	return &store{
		dataPerSource:       map[netip.Addr]*outgoingConnections{},
		cleanupEnabled:      cleanupEnabled,
		cleanupReportedOnly: cleanupReportedOnly,
		cleanupPeriod:       cleanupPeriod,
		traceCleanups:       traceCleanups,
		lookupTable:         lookupTable,
		log:                 logrus.WithField("component", "closed-connections-store"),
	}
}

func (fs *store) StartCleanupWorker() {
	if !fs.cleanupEnabled {
		return
	}
	fs.cleanupTicker = time.NewTicker(fs.cleanupPeriod)
	go func() {
		fs.log.Infof("Starting cleanup agent for closed connections...")
		for {
			select {
			case t := <-fs.cleanupTicker.C:
				fs.performCleanup(t)
			}
		}
	}()
}

func (fs *store) performCleanup(t time.Time) {
	fs.log.Infof("Performing cleanup of closed connections...")
	fs.dataPerSourceLock.Lock()
	defer fs.dataPerSourceLock.Unlock()
	now := time.Now()
	connections := 0
	cleanedConnections := 0
	for src, v := range fs.dataPerSource {
		for _, m := range []map[netip.Addr]*connectionData{v.realDestinations, v.serviceDestinations} {
			for dst, cd := range m {
				connections++
				if (!fs.cleanupReportedOnly || cd.reported) &&
					now.Sub(cd.lastUpdate) > fs.cleanupPeriod &&
					!fs.lookupTable.Exists(net.IP(src.AsSlice()), net.IP(dst.AsSlice())) {
					if fs.traceCleanups {
						fs.log.Infof("Cleaning up connection: %s->%s, sent/received %d/%d (%d/%d), flows %d, last update %s",
							src, dst, cd.sentBytes, cd.receivedBytes, cd.sentPackets, cd.receivedPackets, cd.flowCount, cd.lastUpdate)
					}
					cleanedConnections++
					delete(m, dst)
				}
			}
		}
	}
	fs.log.Infof("Cleaned up of %d closed connections (%d total) in %s (locking time: %s).", cleanedConnections, connections, time.Now().Sub(t), now.Sub(t))
}

func (fs *store) StopCleanupWorker() {
	if !fs.cleanupEnabled {
		return
	}
	fs.log.Infof("Stopping cleanup agent for closed connections...")
	fs.cleanupTicker.Stop()
}

func (fs *store) StoreFlow(src, dst, svcDst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64, t time.Time) error {
	srcKey, ok := utils.ConvertIP(src)
	if !ok {
		return fmt.Errorf("error while converting source ip address '%s' during storing of flow: expected byte length 4 or 16, but got %d", src, len(*src))
	}
	fs.dataPerSourceLock.Lock()
	defer fs.dataPerSourceLock.Unlock()
	connections := fs.getOrCreateOutgoingConnections(srcKey)
	if err := fs.storeFlowForDestination(src, dst, sentBytes, receivedBytes, sentPackets, receivedPackets, t, connections.realDestinations); err != nil {
		return err
	}
	if !dst.Equal(*svcDst) {
		if err := fs.storeFlowForDestination(src, svcDst, sentBytes, receivedBytes, sentPackets, receivedPackets, t, connections.serviceDestinations); err != nil {
			return err
		}
	}
	return nil
}

func (fs *store) storeFlowForDestination(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64, t time.Time, destinationData map[netip.Addr]*connectionData) error {
	dstKey, ok := utils.ConvertIP(dst)
	if !ok {
		return fmt.Errorf("error while converting destination ip address '%s' during storing of flow: expected byte length 4 or 16, but got %d", dst, len(*dst))
	}
	data := getOrCreateConnectionData(destinationData, dstKey)
	data.sentBytes += sentBytes
	data.receivedBytes += receivedBytes
	data.sentPackets += sentPackets
	data.receivedPackets += receivedPackets
	data.flowCount += 1
	if t.IsZero() {
		data.lastUpdate = time.Now()
	} else {
		data.lastUpdate = t
	}
	data.reported = false
	return nil
}

func (fs *store) getOrCreateOutgoingConnections(ip netip.Addr) *outgoingConnections {
	result, exists := fs.dataPerSource[ip]
	if !exists {
		result = &outgoingConnections{
			realDestinations:    map[netip.Addr]*connectionData{},
			serviceDestinations: map[netip.Addr]*connectionData{},
		}
		fs.dataPerSource[ip] = result
	}
	return result
}

func getOrCreateConnectionData(m map[netip.Addr]*connectionData, ip netip.Addr) *connectionData {
	result, exists := m[ip]
	if !exists {
		result = &connectionData{}
		m[ip] = result
	}
	return result
}

func (fs *store) IterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error {
	return fs.iterateConnections(callback, func(connections *outgoingConnections) map[netip.Addr]*connectionData {
		return connections.realDestinations
	})
}

func (fs *store) IterateServiceConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error {
	return fs.iterateConnections(callback, func(connections *outgoingConnections) map[netip.Addr]*connectionData {
		return connections.serviceDestinations
	})
}

func (fs *store) iterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error, destinations func(connections *outgoingConnections) map[netip.Addr]*connectionData) error {
	fs.dataPerSourceLock.Lock()
	defer fs.dataPerSourceLock.Unlock()
	for src, value := range fs.dataPerSource {
		srcIP := net.IP(src.AsSlice())
		for dst, data := range destinations(value) {
			dstIP := net.IP(dst.AsSlice())
			if err := callback(&srcIP, &dstIP, data.sentBytes, data.receivedBytes, data.sentPackets, data.receivedPackets, data.flowCount); err != nil {
				return err
			}
			data.reported = true
		}
	}
	return nil
}
