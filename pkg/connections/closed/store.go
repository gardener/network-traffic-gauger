// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package closed

import (
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/gardener/network-traffic-gauger/pkg/utils"
)

type Store interface {
	StoreFlow(src, dst, svcDst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error
	IterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error
	IterateServiceConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error
}

type store struct {
	dataPerSource     map[netip.Addr]*outgoingConnections
	dataPerSourceLock sync.RWMutex
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
}

func NewStore() Store {
	return &store{
		dataPerSource: map[netip.Addr]*outgoingConnections{},
	}
}

func (fs *store) StoreFlow(src, dst, svcDst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error {
	srcKey, ok := utils.ConvertIP(src)
	if !ok {
		return fmt.Errorf("error while converting source ip address '%s' during storing of flow: expected byte length 4 or 16, but got %d", src, len(*src))
	}
	fs.dataPerSourceLock.Lock()
	defer fs.dataPerSourceLock.Unlock()
	connections := fs.getOrCreateOutgoingConnections(srcKey)
	if err := fs.storeFlowForDestination(src, dst, sentBytes, receivedBytes, sentPackets, receivedPackets, connections.realDestinations); err != nil {
		return err
	}
	if !dst.Equal(*svcDst) {
		if err := fs.storeFlowForDestination(src, svcDst, sentBytes, receivedBytes, sentPackets, receivedPackets, connections.serviceDestinations); err != nil {
			return err
		}
	}
	return nil
}

func (fs *store) storeFlowForDestination(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64, destinationData map[netip.Addr]*connectionData) error {
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
	fs.dataPerSourceLock.RLock()
	defer fs.dataPerSourceLock.RUnlock()
	for src, value := range fs.dataPerSource {
		srcIP := net.IP(src.AsSlice())
		for dst, data := range destinations(value) {
			dstIP := net.IP(dst.AsSlice())
			if err := callback(&srcIP, &dstIP, data.sentBytes, data.receivedBytes, data.sentPackets, data.receivedPackets, data.flowCount); err != nil {
				return err
			}
		}
	}
	return nil
}
