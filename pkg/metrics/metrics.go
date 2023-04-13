// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"

	"github.com/gardener/network-traffic-gauger/pkg/connections/active"
	"github.com/gardener/network-traffic-gauger/pkg/connections/closed"
	"github.com/gardener/network-traffic-gauger/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type MetricsServer interface {
	ServiceMetrics()
}

type metricsServer struct {
	activeConnectionsStore            active.Store
	closedConnectionsStore            closed.Store
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

func NewMetricsServer(activeConnectionsStore active.Store, closedConnectionsStore closed.Store, port int, enableErrorLog bool, enableServiceMetrics bool, enableByteMetrics bool, enablePacketMetrics bool, enableFlowCountMetrics bool, reportErrorsDuringCollection bool) MetricsServer {
	return &metricsServer{
		activeConnectionsStore:            activeConnectionsStore,
		closedConnectionsStore:            closedConnectionsStore,
		port:                              port,
		enableErrorLog:                    enableErrorLog,
		enableServiceMetrics:              enableServiceMetrics,
		enableByteMetrics:                 enableByteMetrics,
		enablePacketMetrics:               enablePacketMetrics,
		enableFlowCountMetrics:            enableFlowCountMetrics,
		reportErrorsDuringCollection:      reportErrorsDuringCollection,
		sentBytesDescription:              prometheus.NewDesc("network_transmit_bytes_total", "Total number of bytes transmitted.", []string{"src", "dst"}, nil),
		receivedBytesDescription:          prometheus.NewDesc("network_receive_bytes_total", "Total number of bytes received.", []string{"src", "dst"}, nil),
		sentPacketsDescription:            prometheus.NewDesc("network_transmit_packets_total", "Total number of packets transmitted.", []string{"src", "dst"}, nil),
		receivedPacketsDescription:        prometheus.NewDesc("network_receive_packets_total", "Total number of packets received.", []string{"src", "dst"}, nil),
		flowCountDescription:              prometheus.NewDesc("network_flow_total", "Total number of network flows.", []string{"src", "dst"}, nil),
		serviceSentBytesDescription:       prometheus.NewDesc("service_network_transmit_bytes_total", "Total number of bytes transmitted to a service.", []string{"src", "dst"}, nil),
		serviceReceivedBytesDescription:   prometheus.NewDesc("service_network_receive_bytes_total", "Total number of bytes received from a service.", []string{"src", "dst"}, nil),
		serviceSentPacketsDescription:     prometheus.NewDesc("service_network_transmit_packets_total", "Total number of packets transmitted to a service.", []string{"src", "dst"}, nil),
		serviceReceivedPacketsDescription: prometheus.NewDesc("service_network_receive_packets_total", "Total number of packets received from a service.", []string{"src", "dst"}, nil),
		serviceFlowCountDescription:       prometheus.NewDesc("service_network_flow_total", "Total number of network flows to a service.", []string{"src", "dst"}, nil),
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
	log.Fatalf("error running metrics server: %v", http.ListenAndServe(fmt.Sprintf(":%d", ms.port), nil))
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
	sentBytesDescription, receivedBytesDescription, sentPacketsDescription, receivedPacketsDescription, flowCountDescription *prometheus.Desc) {
	// Prepare open connections for fast lookup during closed connections store iteration
	openConnections := map[netip.Addr]map[netip.Addr]*flow{}
	if err := activeConnectionsStoreIteration(func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets uint64) error {
		srcIp, ok := utils.ConvertIP(src)
		if !ok {
			return fmt.Errorf("error while converting source ip '%s' during metrics collection from open connections: expected byte length 4 or 16, but got %d", src.String(), len(*src))
		}
		dstMap, exists := openConnections[srcIp]
		if !exists {
			dstMap = map[netip.Addr]*flow{}
			openConnections[srcIp] = dstMap
		}
		dstIp, ok := utils.ConvertIP(dst)
		if !ok {
			return fmt.Errorf("error while converting destination ip '%s' during metrics collection from open connections: expected byte length 4 or 16, but got %d", dst.String(), len(*dst))
		}
		f, exists := dstMap[dstIp]
		if !exists {
			f = &flow{}
			dstMap[dstIp] = f
		}
		f.sentBytes += sentBytes
		f.receivedBytes += receivedBytes
		f.sentPackets += sentPackets
		f.receivedPackets += receivedPackets
		f.count += 1
		return nil
	}); err != nil {
		ms.reportError(metricsChannel, err)
	}

	// Create metrics from the closed connections store using the open connection data if available
	if err := closedConnectionsStoreIteration(func(src, dst *net.IP, sentBytes, receivedBytes, sentPackets, receivedPackets, count uint64) error {
		// Check for open connection to add the metrics
		srcIp, ok := utils.ConvertIP(src)
		if !ok {
			return fmt.Errorf("error while converting source ip '%s' during metrics collection from closed connections: expected byte length 4 or 16, but got %d", src.String(), len(*src))
		}
		dstMap, exists := openConnections[srcIp]
		if exists {
			dstIp, ok := utils.ConvertIP(dst)
			if !ok {
				return fmt.Errorf("error while converting destination ip '%s' during metrics collection from closed connections: expected byte length 4 or 16, but got %d", dst.String(), len(*dst))
			}
			f, exists := dstMap[dstIp]
			if exists {
				sentBytes += f.sentBytes
				receivedBytes += f.receivedBytes
				sentPackets += f.sentPackets
				receivedPackets += f.receivedPackets
				count += f.count
			}
		}

		// Emit the metrics depending on the configuration
		if ms.enableByteMetrics {
			metricsChannel <- prometheus.MustNewConstMetric(sentBytesDescription, prometheus.CounterValue, float64(sentBytes), src.String(), dst.String())
			metricsChannel <- prometheus.MustNewConstMetric(receivedBytesDescription, prometheus.CounterValue, float64(receivedBytes), src.String(), dst.String())
		}
		if ms.enablePacketMetrics {
			metricsChannel <- prometheus.MustNewConstMetric(sentPacketsDescription, prometheus.CounterValue, float64(sentPackets), src.String(), dst.String())
			metricsChannel <- prometheus.MustNewConstMetric(receivedPacketsDescription, prometheus.CounterValue, float64(receivedPackets), src.String(), dst.String())
		}
		if ms.enableFlowCountMetrics {
			metricsChannel <- prometheus.MustNewConstMetric(flowCountDescription, prometheus.CounterValue, float64(count), src.String(), dst.String())
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
