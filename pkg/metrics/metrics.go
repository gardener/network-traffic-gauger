// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/gardener/network-traffic-gauger/pkg/cluster"
	"github.com/gardener/network-traffic-gauger/pkg/connections/active"
	"github.com/gardener/network-traffic-gauger/pkg/connections/closed"
	"github.com/gardener/network-traffic-gauger/pkg/utils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type Server interface {
	ServiceMetrics()
}

type metricsServer struct {
	activeConnectionsStore            active.Store
	closedConnectionsStore            closed.Store
	clusterInfo                       cluster.Info
	port                              int
	enableErrorLog                    bool
	enableServiceMetrics              bool
	enableByteMetrics                 bool
	enablePacketMetrics               bool
	enableFlowCountMetrics            bool
	reportErrorsDuringCollection      bool
	sentBytesDescription              *prometheus.Desc
	receivedBytesDescription          *prometheus.Desc
	sentPacketsDescription            *prometheus.Desc
	receivedPacketsDescription        *prometheus.Desc
	flowCountDescription              *prometheus.Desc
	serviceSentBytesDescription       *prometheus.Desc
	serviceReceivedBytesDescription   *prometheus.Desc
	serviceSentPacketsDescription     *prometheus.Desc
	serviceReceivedPacketsDescription *prometheus.Desc
	serviceFlowCountDescription       *prometheus.Desc
}

type flow struct {
	sentBytes       uint64
	receivedBytes   uint64
	sentPackets     uint64
	receivedPackets uint64
	count           uint64
}

func NewServer(activeConnectionsStore active.Store, closedConnectionsStore closed.Store, clusterInfo cluster.Info, port int, enableErrorLog bool, enableServiceMetrics bool, enableByteMetrics bool, enablePacketMetrics bool, enableFlowCountMetrics bool, reportErrorsDuringCollection bool) Server {
	return &metricsServer{
		activeConnectionsStore:            activeConnectionsStore,
		closedConnectionsStore:            closedConnectionsStore,
		clusterInfo:                       clusterInfo,
		port:                              port,
		enableErrorLog:                    enableErrorLog,
		enableServiceMetrics:              enableServiceMetrics,
		enableByteMetrics:                 enableByteMetrics,
		enablePacketMetrics:               enablePacketMetrics,
		enableFlowCountMetrics:            enableFlowCountMetrics,
		reportErrorsDuringCollection:      reportErrorsDuringCollection,
		sentBytesDescription:              prometheus.NewDesc("network_transmit_bytes_total", "Total number of bytes transmitted.", []string{"src", "dst", "type"}, nil),
		receivedBytesDescription:          prometheus.NewDesc("network_receive_bytes_total", "Total number of bytes received.", []string{"src", "dst", "type"}, nil),
		sentPacketsDescription:            prometheus.NewDesc("network_transmit_packets_total", "Total number of packets transmitted.", []string{"src", "dst", "type"}, nil),
		receivedPacketsDescription:        prometheus.NewDesc("network_receive_packets_total", "Total number of packets received.", []string{"src", "dst", "type"}, nil),
		flowCountDescription:              prometheus.NewDesc("network_flow_total", "Total number of network flows.", []string{"src", "dst", "type"}, nil),
		serviceSentBytesDescription:       prometheus.NewDesc("service_network_transmit_bytes_total", "Total number of bytes transmitted to a service.", []string{"src", "dst", "type"}, nil),
		serviceReceivedBytesDescription:   prometheus.NewDesc("service_network_receive_bytes_total", "Total number of bytes received from a service.", []string{"src", "dst", "type"}, nil),
		serviceSentPacketsDescription:     prometheus.NewDesc("service_network_transmit_packets_total", "Total number of packets transmitted to a service.", []string{"src", "dst", "type"}, nil),
		serviceReceivedPacketsDescription: prometheus.NewDesc("service_network_receive_packets_total", "Total number of packets received from a service.", []string{"src", "dst", "type"}, nil),
		serviceFlowCountDescription:       prometheus.NewDesc("service_network_flow_total", "Total number of network flows to a service.", []string{"src", "dst", "type"}, nil),
	}
}

func (ms *metricsServer) ServiceMetrics() {
	log := logrus.WithField("component", "metrics")
	if ms.port == 0 {
		log.Infof("Metrics server disabled.")
		return
	}
	log.Infof("Serving metrics on port %d...", ms.port)
	var errorLog promhttp.Logger
	if ms.enableErrorLog {
		errorLog = log
	}
	registry := prometheus.NewRegistry()
	registry.MustRegister(ms)
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry, ErrorLog: errorLog}))
	log.Fatalf("error running metrics server: %v", (&http.Server{
		Addr:              fmt.Sprintf(":%d", ms.port),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}).ListenAndServe())
}

func (ms *metricsServer) Describe(descriptionChannel chan<- *prometheus.Desc) {
	if ms.enableByteMetrics {
		descriptionChannel <- ms.sentBytesDescription
		descriptionChannel <- ms.receivedBytesDescription
	}
	if ms.enablePacketMetrics {
		descriptionChannel <- ms.sentPacketsDescription
		descriptionChannel <- ms.receivedPacketsDescription
	}
	if ms.enableFlowCountMetrics {
		descriptionChannel <- ms.flowCountDescription
	}
	if ms.enableServiceMetrics {
		if ms.enableByteMetrics {
			descriptionChannel <- ms.serviceSentBytesDescription
			descriptionChannel <- ms.serviceReceivedBytesDescription
		}
		if ms.enablePacketMetrics {
			descriptionChannel <- ms.serviceSentPacketsDescription
			descriptionChannel <- ms.serviceReceivedPacketsDescription
		}
		if ms.enableFlowCountMetrics {
			descriptionChannel <- ms.serviceFlowCountDescription
		}
	}
}

func (ms *metricsServer) Collect(metricsChannel chan<- prometheus.Metric) {
	ms.collect(metricsChannel, ms.activeConnectionsStore.IterateConnections, ms.closedConnectionsStore.IterateConnections, ms.sentBytesDescription, ms.receivedBytesDescription, ms.sentPacketsDescription, ms.receivedPacketsDescription, ms.flowCountDescription)
	if ms.enableServiceMetrics {
		ms.collect(metricsChannel, ms.activeConnectionsStore.IterateServiceConnections, ms.closedConnectionsStore.IterateServiceConnections, ms.serviceSentBytesDescription, ms.serviceReceivedBytesDescription, ms.serviceSentPacketsDescription, ms.serviceReceivedPacketsDescription, ms.serviceFlowCountDescription)
	}
}

func (ms *metricsServer) collect(metricsChannel chan<- prometheus.Metric,
	activeConnectionsStoreIteration func(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error) error,
	closedConnectionsStoreIteration func(callback func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error) error,
	sentBytesDescription, receivedBytesDescription, sentPacketsDescription, receivedPacketsDescription, flowCountDescription *prometheus.Desc,
) {
	// Prepare open connections for fast lookup during closed connections store iteration
	openConnections := map[netip.Addr]map[netip.Addr]*flow{}
	if err := activeConnectionsStoreIteration(func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error {
		srcIP, ok := utils.ConvertIP(src)
		if !ok {
			return fmt.Errorf("error while converting source ip '%s' during metrics collection from open connections: expected byte length 4 or 16, but got %d", src.String(), len(*src))
		}
		dstMap, exists := openConnections[srcIP]
		if !exists {
			dstMap = map[netip.Addr]*flow{}
			openConnections[srcIP] = dstMap
		}
		dstIP, ok := utils.ConvertIP(dst)
		if !ok {
			return fmt.Errorf("error while converting destination ip '%s' during metrics collection from open connections: expected byte length 4 or 16, but got %d", dst.String(), len(*dst))
		}
		f, exists := dstMap[dstIP]
		if !exists {
			f = &flow{}
			dstMap[dstIP] = f
		}
		f.sentBytes += sentBytes
		f.receivedBytes += receivedBytes
		f.sentPackets += sentPackets
		f.receivedPackets += receivedPackets
		f.count++
		return nil
	}); err != nil {
		ms.reportError(metricsChannel, err)
	}

	// Create metrics from the closed connections store using the open connection data if available
	if err := closedConnectionsStoreIteration(func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error {
		// Check for open connection to add the metrics
		srcIP, ok := utils.ConvertIP(src)
		if !ok {
			return fmt.Errorf("error while converting source ip '%s' during metrics collection from closed connections: expected byte length 4 or 16, but got %d", src.String(), len(*src))
		}
		dstIP, ok := utils.ConvertIP(dst)
		if !ok {
			return fmt.Errorf("error while converting destination ip '%s' during metrics collection from closed connections: expected byte length 4 or 16, but got %d", dst.String(), len(*dst))
		}
		dstMap, exists := openConnections[srcIP]
		if exists {
			f, exists := dstMap[dstIP]
			if exists {
				sentBytes += f.sentBytes
				receivedBytes += f.receivedBytes
				sentPackets += f.sentPackets
				receivedPackets += f.receivedPackets
				count += f.count
			}
		}

		// Check if connection is local, cluster or internet
		connectionType := ms.determineConnectionType(srcIP, dstIP)

		// Emit the metrics depending on the configuration
		if ms.enableByteMetrics {
			metricsChannel <- prometheus.MustNewConstMetric(sentBytesDescription, prometheus.CounterValue, float64(sentBytes), src.String(), dst.String(), connectionType)
			metricsChannel <- prometheus.MustNewConstMetric(receivedBytesDescription, prometheus.CounterValue, float64(receivedBytes), src.String(), dst.String(), connectionType)
		}
		if ms.enablePacketMetrics {
			metricsChannel <- prometheus.MustNewConstMetric(sentPacketsDescription, prometheus.CounterValue, float64(sentPackets), src.String(), dst.String(), connectionType)
			metricsChannel <- prometheus.MustNewConstMetric(receivedPacketsDescription, prometheus.CounterValue, float64(receivedPackets), src.String(), dst.String(), connectionType)
		}
		if ms.enableFlowCountMetrics {
			metricsChannel <- prometheus.MustNewConstMetric(flowCountDescription, prometheus.CounterValue, float64(count), src.String(), dst.String(), connectionType)
		}
		return nil
	}); err != nil {
		ms.reportError(metricsChannel, err)
	}
}

func (ms *metricsServer) reportError(metricsChannel chan<- prometheus.Metric, err error) {
	if ms.reportErrorsDuringCollection {
		metricsChannel <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
	}
}

func (ms *metricsServer) determineConnectionType(src netip.Addr, dst netip.Addr) string {
	srcLocal := ms.clusterInfo.IsLocalAddress(src)
	dstLocal := ms.clusterInfo.IsLocalAddress(dst)
	if srcLocal && dstLocal {
		return "local"
	}
	if src.IsLinkLocalUnicast() || src.IsLinkLocalMulticast() || dst.IsLinkLocalUnicast() || dst.IsLinkLocalMulticast() {
		return "link-local"
	}
	srcCluster := ms.clusterInfo.IsInClusterRange(src)
	dstCluster := ms.clusterInfo.IsInClusterRange(dst)
	if (srcLocal && dstCluster) || (srcCluster && dstLocal) {
		return "cluster"
	}
	return "internet"
}
