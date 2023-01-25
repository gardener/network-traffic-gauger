// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package filestore

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/gardener/network-traffic-gauger/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	dataSuffix         = ".data"
	serviceSuffix      = ".svc"
	fileMarkerByte1    = 0x01 // version
	fileMarkerByte2    = 0xFF
	fileMarkerByte3    = 0xDA
	fileMarkerByte4    = 0xBA
	defaultPermissions = 0644
)

var (
	fileMarker = [...]byte{fileMarkerByte1, fileMarkerByte2, fileMarkerByte3, fileMarkerByte4}
)

type FileStore interface {
	Load() error
	StoreFlow(src, dst, svcDst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error
	IterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error
	IterateServiceConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error
}

type fileStore struct {
	directory         string
	dataPerSource     map[netip.Addr]*outgoingConnections
	dataPerSourceLock sync.RWMutex
}

type outgoingConnections struct {
	realDestinations    map[netip.Addr]*connectionData
	serviceDestinations map[netip.Addr]*connectionData
}

type connectionData struct {
	fileOffset      int64
	sentBytes       uint64
	receivedBytes   uint64
	sentPackets     uint64
	receivedPackets uint64
	flowCount       uint64
}

type abortableCloser struct {
	file *os.File
}

func (ac *abortableCloser) Close() error {
	if ac.file != nil {
		return ac.file.Close()
	}
	return nil
}

func EnsureDirExists(dir string) error {
	return os.MkdirAll(dir, defaultPermissions)
}

func NewFileStore(dir string) FileStore {
	return &fileStore{
		directory:     dir,
		dataPerSource: map[netip.Addr]*outgoingConnections{},
	}
}

func (fs *fileStore) Load() error {
	log := logrus.WithField("component", "filestore")
	log.Infof("Trying to load persistent connection data...")
	if fi, err := os.Stat(fs.directory); err != nil {
		return fmt.Errorf("error trying to access file store dir '%s': %w", fs.directory, err)
	} else if !fi.IsDir() {
		return fmt.Errorf("error file store dir '%s' is not a directory", fs.directory)
	}
	files, err := os.ReadDir(fs.directory)
	if err != nil {
		return fmt.Errorf("error while reading content of file store directory '%s': %w", fs.directory, err)
	}
	fs.dataPerSourceLock.Lock()
	defer fs.dataPerSourceLock.Unlock()
	count := 0
	for _, f := range files {
		var nameWithoutSuffix string
		var insertFunc func(connections *outgoingConnections, data map[netip.Addr]*connectionData)
		if strings.HasSuffix(f.Name(), dataSuffix) {
			nameWithoutSuffix = f.Name()[:len(f.Name())-len(dataSuffix)]
			insertFunc = func(connections *outgoingConnections, data map[netip.Addr]*connectionData) {
				connections.realDestinations = data
			}
		} else if strings.HasSuffix(f.Name(), serviceSuffix) {
			nameWithoutSuffix = f.Name()[:len(f.Name())-len(serviceSuffix)]
			insertFunc = func(connections *outgoingConnections, data map[netip.Addr]*connectionData) {
				connections.serviceDestinations = data
			}
		} else {
			// Not a file belonging to the file store persistance
			continue
		}
		ip := net.ParseIP(nameWithoutSuffix)
		if ip == nil {
			return fmt.Errorf("error while parsing ip address from file name '%s'", f.Name())
		}
		netIp, ok := utils.ConvertIP(&ip)
		if !ok {
			return fmt.Errorf("error while converting ip address from file name '%s'", f.Name())
		}
		data, err := loadFile(path.Join(fs.directory, f.Name()))
		if err != nil {
			return err
		}
		insertFunc(fs.getOrCreateOutgoingConnections(netIp), data)
		count++
	}
	if count == 0 {
		log.Infof("No persistent connection data found in directory '%s'.", fs.directory)
	} else {
		log.Infof("Successfully loaded persistent connection data from %d source files from directory '%s'.", count, fs.directory)
	}
	return nil
}

func loadFile(name string) (map[netip.Addr]*connectionData, error) {
	log := logrus.WithField("component", "filestore")
	log.Infof("Trying to load persistent connection data from '%s'...", name)
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("error while opening file '%s' during load: %w", name, err)
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("error while retrieving file info of file '%s': %w", name, err)
	}
	size := fi.Size()
	if err := checkFileMarker(file); err != nil {
		return nil, err
	}
	result := map[netip.Addr]*connectionData{}
	count := 0
	currentPosition, err := file.Seek(int64(len(fileMarker)), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("error while getting current file position of file '%s': %w", name, err)
	}
	for currentPosition < size {
		ip, n, err := deserializeIP(file)
		if err != nil {
			return nil, fmt.Errorf("error while deserializing ip address during load of file '%s': %w", name, err)
		}
		currentPosition += int64(n)
		ipKey, ok := utils.ConvertIP(ip)
		if !ok {
			return nil, fmt.Errorf("error while converting ip address '%s' during load of file '%s': expected byte length 4 or 16, but got %d", ip, name, len(*ip))
		}
		cd := &connectionData{}
		cd.sentBytes, cd.receivedBytes, cd.sentPackets, cd.receivedPackets, cd.flowCount, n, err = deserializeData(file)
		if err != nil {
			return nil, fmt.Errorf("error while deserializing connection data for destination '%s' during load of file '%s': %w", ip, name, err)
		}
		cd.fileOffset = currentPosition
		currentPosition += int64(n)
		if _, exists := result[ipKey]; exists {
			log.Warnf("Duplicate entry detected: data for ip address '%s' found more than once during load of file '%s'", ip, name)
		}
		result[ipKey] = cd
		count++
	}
	if count == 0 {
		log.Infof("No persistent connection data found in '%s'.", name)
	} else {
		log.Infof("Successfully loaded persistent connection data from '%s' containing information about %d destinations.", name, count)
	}
	return result, nil
}

func checkFileMarker(file *os.File) error {
	marker := []byte{0, 0, 0, 0}
	if _, err := file.ReadAt(marker, 0); err != nil {
		return fmt.Errorf("error while reading file marker of file '%s': %w", file.Name(), err)
	} else if !bytes.Equal(fileMarker[:], marker) {
		return fmt.Errorf("incorrect file marker in file '%s': 0x%x%x%x%x instead of 0x%x%x%x%x",
			file.Name(), marker[0], marker[1], marker[2], marker[3], fileMarker[0], fileMarker[1], fileMarker[2], fileMarker[3])
	}
	return nil
}

func (fs *fileStore) StoreFlow(src, dst, svcDst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error {
	srcKey, ok := utils.ConvertIP(src)
	if !ok {
		return fmt.Errorf("error while converting source ip address '%s' during storing of flow: expected byte length 4 or 16, but got %d", src, len(*src))
	}
	fs.dataPerSourceLock.Lock()
	defer fs.dataPerSourceLock.Unlock()
	connections := fs.getOrCreateOutgoingConnections(srcKey)
	if err := fs.storeFlowForDestination(src, dst, sentBytes, receivedBytes, sentPackets, receivedPackets, connections.realDestinations, dataSuffix); err != nil {
		return err
	}
	if !dst.Equal(*svcDst) {
		if err := fs.storeFlowForDestination(src, svcDst, sentBytes, receivedBytes, sentPackets, receivedPackets, connections.serviceDestinations, serviceSuffix); err != nil {
			return err
		}
	}
	return nil
}

func (fs *fileStore) storeFlowForDestination(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64, destinationData map[netip.Addr]*connectionData, suffix string) error {
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
	dataFile, err := fs.openOrCreateFile(src, suffix)
	if err != nil {
		return err
	}
	defer dataFile.Close()
	if data.fileOffset == 0 {
		// Entry needs to be appended and file offset needs to be calculated
		fi, err := dataFile.Stat()
		if err != nil {
			return fmt.Errorf("error while retrieving file info of file '%s': %w", dataFile.Name(), err)
		}
		fileSize := fi.Size()
		if ipBytes, err := serializeIP(dst); err != nil {
			return fmt.Errorf("error while serializing ip address '%s' during storing of flow: %w", dst, err)
		} else {
			n, err := dataFile.WriteAt(ipBytes, fileSize)
			if err != nil {
				return fmt.Errorf("error while writing ip address '%s' to file '%s' during storing of flow: %w", dst, dataFile.Name(), err)
			}
			data.fileOffset = fileSize + int64(n)
		}
	}
	if dataBytes, err := serializeData(data.sentBytes, data.receivedBytes, data.sentPackets, data.receivedPackets, data.flowCount); err != nil {
		return fmt.Errorf("error while serializing flow data during storing of flow: %w", err)
	} else {
		_, err := dataFile.WriteAt(dataBytes, data.fileOffset)
		if err != nil {
			return fmt.Errorf("error while writing flow data to file '%s' during storing of flow: %w", dataFile.Name(), err)
		}
	}
	return nil
}

func (fs *fileStore) getOrCreateOutgoingConnections(ip netip.Addr) *outgoingConnections {
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

func (fs *fileStore) openOrCreateFile(ip *net.IP, extension string) (*os.File, error) {
	name := ip.String() + extension
	file, err := os.OpenFile(path.Join(fs.directory, name), os.O_RDWR|os.O_CREATE, defaultPermissions)
	if err != nil {
		return nil, fmt.Errorf("error while opening/creating file '%s': %w", name, err)
	}
	ac := &abortableCloser{file: file}
	defer ac.Close()
	fi, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("error while retrieving file info of file '%s': %w", name, err)
	}
	if fi.Size() == 0 {
		// File was just created => store file marker
		if _, err := file.WriteAt(fileMarker[:], 0); err != nil {
			return nil, fmt.Errorf("error while writing file marker into file '%s': %w", name, err)
		}
	}
	if err := checkFileMarker(file); err != nil {
		return nil, err
	}
	ac.file = nil
	return file, nil
}

func (fs *fileStore) IterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error {
	return fs.iterateConnections(callback, func(connections *outgoingConnections) map[netip.Addr]*connectionData {
		return connections.realDestinations
	})
}

func (fs *fileStore) IterateServiceConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error {
	return fs.iterateConnections(callback, func(connections *outgoingConnections) map[netip.Addr]*connectionData {
		return connections.serviceDestinations
	})
}

func (fs *fileStore) iterateConnections(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error, destinations func(connections *outgoingConnections) map[netip.Addr]*connectionData) error {
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
