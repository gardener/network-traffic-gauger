// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package filestore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
)

var (
	byteOrder = binary.LittleEndian
)

func serializeData(sentBytes, receivedBytes, sentPackets, receivedPackets, flowCount uint64) ([]byte, error) {
	var buffer bytes.Buffer
	for _, data := range []uint64{sentBytes, receivedBytes, sentPackets, receivedPackets, flowCount} {
		if err := binary.Write(&buffer, byteOrder, data); err != nil {
			return nil, fmt.Errorf("error while serializing data: %w", err)
		}
	}
	return buffer.Bytes(), nil
}

func deserializeData(reader io.Reader) (sentBytes, receivedBytes, sentPackets, receivedPackets, flowCount uint64, n int, e error) {
	for _, data := range []*uint64{&sentBytes, &receivedBytes, &sentPackets, &receivedPackets, &flowCount} {
		if err := binary.Read(reader, byteOrder, data); err != nil {
			e = fmt.Errorf("error while deserializing data: %w", err)
			return
		}
		n += 8
	}
	return
}

func serializeIP(ip *net.IP) ([]byte, error) {
	var buffer bytes.Buffer
	length := len(*ip)
	if length > math.MaxUint8 {
		return nil, fmt.Errorf("error while serializing ip address '%s': address too long (%d bytes)", *ip, length)
	}
	if err := binary.Write(&buffer, byteOrder, uint8(length)); err != nil {
		return nil, fmt.Errorf("error while serializing length of ip address '%s': %w", *ip, err)
	}
	buffer.Write(*ip)
	return buffer.Bytes(), nil
}

func deserializeIP(reader io.Reader) (*net.IP, int, error) {
	var length uint8
	if err := binary.Read(reader, byteOrder, &length); err != nil {
		return nil, 0, fmt.Errorf("error while deserializing length of ip address: %w", err)
	}
	ip := make(net.IP, length)
	if n, err := reader.Read(ip); err != nil {
		return nil, 0, fmt.Errorf("error while deserializing ip address: %w", err)
	} else {
		return &ip, n + 1, nil
	}
}
