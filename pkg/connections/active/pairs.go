// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package active

import (
	"net"
	"net/netip"
	"sync"

	"github.com/gardener/network-traffic-gauger/pkg/utils"
	"github.com/sirupsen/logrus"
)

type ActiveConnectionPairs interface {
	Add(src net.IP, dst net.IP, svcDst net.IP)
	Remove(src net.IP, dst net.IP, svcDst net.IP)
	Exists(src net.IP, dst net.IP) bool
}

type pair struct {
	src netip.Addr
	dst netip.Addr
}

type activeConnectionPairs struct {
	connections map[pair]uint64
	lock        sync.RWMutex
	log         *logrus.Entry
}

func NewLookupTable() ActiveConnectionPairs {
	return &activeConnectionPairs{
		connections: map[pair]uint64{},
		log:         logrus.WithField("component", "active-connections-lookup-table"),
	}
}

func (acp *activeConnectionPairs) Add(src net.IP, dst net.IP, svcDst net.IP) {
	key, ok := acp.convert(src, dst)
	if !ok {
		return
	}
	svcKey, ok := acp.convert(src, svcDst)
	if !ok {
		return
	}
	acp.lock.Lock()
	defer acp.lock.Unlock()
	acp.add(key)
	if !dst.Equal(svcDst) {
		acp.add(svcKey)
	}
}

func (acp *activeConnectionPairs) add(key pair) {
	count, exists := acp.connections[key]
	if !exists {
		count = 0
	}
	count++
	acp.connections[key] = count
}

func (acp *activeConnectionPairs) Remove(src net.IP, dst net.IP, svcDst net.IP) {
	key, ok := acp.convert(src, dst)
	if !ok {
		return
	}
	svcKey, ok := acp.convert(src, svcDst)
	if !ok {
		return
	}
	acp.lock.Lock()
	defer acp.lock.Unlock()
	acp.remove(key)
	if !dst.Equal(svcDst) {
		acp.remove(svcKey)
	}
}

func (acp *activeConnectionPairs) remove(key pair) {
	count, exists := acp.connections[key]
	if !exists {
		return
	}
	if count <= 1 {
		delete(acp.connections, key)
	} else {
		acp.connections[key] = count - 1
	}
}

func (acp *activeConnectionPairs) Exists(src net.IP, dst net.IP) bool {
	key, ok := acp.convert(src, dst)
	if !ok {
		return false
	}
	acp.lock.RLock()
	defer acp.lock.RUnlock()
	_, exists := acp.connections[key]
	return exists
}

func (acp *activeConnectionPairs) convert(src net.IP, dst net.IP) (pair, bool) {
	srcIp, ok := utils.ConvertIP(&src)
	if !ok {
		acp.log.Errorf("Converting source IP address failed, ignoring active connection in lookup table: %s->%s", src, dst)
		return pair{}, false
	}
	dstIp, ok := utils.ConvertIP(&dst)
	if !ok {
		acp.log.Errorf("Converting destination IP address failed, ignoring active connection in lookup table: %s->%s", src, dst)
		return pair{}, false
	}
	return pair{src: srcIp, dst: dstIp}, true
}
