// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"time"

	ct "github.com/florianl/go-conntrack"
	"github.com/gardener/network-traffic-gauger/pkg/filestore"
	"github.com/gardener/network-traffic-gauger/pkg/memorystore"
	"github.com/gardener/network-traffic-gauger/pkg/metrics"
	"github.com/gardener/network-traffic-gauger/pkg/setup"
	"github.com/gardener/network-traffic-gauger/pkg/utils"
	"github.com/mdlayher/netlink"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type runAgentCommand struct {
	netfilterDumpPeriod                 time.Duration
	netfilterEventChannelBufferSize     int
	netfilterEventReceiveBufferSize     int
	netfilterCreateEventsEnabled        bool
	netfilterTraceConnections           bool
	netfilterTraceConnectionChange      bool
	netfilterIgnoreLoopbackTraffic      bool
	netfilterIgnoreBufferErrors         bool
	fileStoreDirectory                  string
	fileStoreChannelBufferSize          int
	fileStoreTraceStores                bool
	metricsPort                         int
	metricsEnableErrorLog               bool
	metricsEnableServiceMetrics         bool
	metricsEnableByteMetrics            bool
	metricsEnablePacketMetrics          bool
	metricsEnableFlowCountMetrics       bool
	metricsReportErrorsDuringCollection bool
}

func CreateRunAgentCmd() *cobra.Command {
	rac := &runAgentCommand{}
	cmd := &cobra.Command{
		Use:   "run-agent",
		Short: "runs network traffic measurement agent",
		Long:  "runs network traffic measurement agent utilizing netfilter connection tracking accounting",
		RunE:  rac.runAgent,
	}
	cmd.Flags().DurationVar(&rac.netfilterDumpPeriod, "dump-period", 1*time.Minute, "time interval between dumping connectrion tracking table")
	cmd.Flags().IntVar(&rac.netfilterEventChannelBufferSize, "event-channel-buffer-size", 1024, "size of the connection tracking table event channel buffer")
	cmd.Flags().IntVar(&rac.netfilterEventReceiveBufferSize, "event-receive-buffer-size", 1024*1024, "size of the receive buffer (in bytes) used for connection tracking events (defaults to 1M)")
	cmd.Flags().BoolVar(&rac.netfilterCreateEventsEnabled, "create-events-enabled", false, "listen for netfilter connection create events")
	cmd.Flags().BoolVar(&rac.netfilterTraceConnections, "trace-connections", false, "trace connections to stderr")
	cmd.Flags().BoolVar(&rac.netfilterTraceConnectionChange, "trace-connection-changes", false, "trace connection changes to stderr")
	cmd.Flags().BoolVar(&rac.netfilterIgnoreLoopbackTraffic, "ignore-loopback-traffic", true, "ignore local traffic on the loopback device")
	cmd.Flags().BoolVar(&rac.netfilterIgnoreBufferErrors, "ignore-event-buffer-errors", false, "ignore errors related to buffer handling of netfilter connection tracking events")
	cmd.Flags().StringVar(&rac.fileStoreDirectory, "file-store-directory", "/var/log/net-gauger", "directory used for storage of flow data")
	cmd.Flags().IntVar(&rac.fileStoreChannelBufferSize, "file-store-channel-buffer-size", 1024, "size of the file store channel buffer")
	cmd.Flags().BoolVar(&rac.fileStoreTraceStores, "trace-file-stores", false, "trace store operations on the file storage")
	cmd.Flags().IntVar(&rac.metricsPort, "metrics-port", 16160, "port to use for serving metrics (set to '0' to disable metrics serving)")
	cmd.Flags().BoolVar(&rac.metricsEnableErrorLog, "enable-metrics-error-log", true, "enable error log in the metrics server")
	cmd.Flags().BoolVar(&rac.metricsEnableServiceMetrics, "enable-service-metrics", true, "enable metrics for kubernetes services")
	cmd.Flags().BoolVar(&rac.metricsEnableByteMetrics, "enable-byte-metrics", true, "enable metrics indicating how many bytes are transmitted/received")
	cmd.Flags().BoolVar(&rac.metricsEnablePacketMetrics, "enable-packet-metrics", true, "enable metrics indicating how many packets are transmitted/received")
	cmd.Flags().BoolVar(&rac.metricsEnableFlowCountMetrics, "enable-flow-count-metrics", true, "enable metrics indicating how many connections are created")
	cmd.Flags().BoolVar(&rac.metricsReportErrorsDuringCollection, "report-errors-during-metrics-collection", true, "enable error reporting during metrics collection, which might interrupt metrics collection")
	return cmd
}

func (rac *runAgentCommand) runAgent(ccmd *cobra.Command, args []string) error {
	log := logrus.WithField("cmd", "run-agent")

	log.Infof("Checking netfilter prerequisites...")
	if enabled, err := setup.CheckNetfilterPrerequisites(); err != nil {
		return fmt.Errorf("checking netfilter prerequisites failed: %w", err)
	} else if !enabled {
		return fmt.Errorf("netfilter prerequisites not available, please run 'net-gauger setup'")
	}
	eventChannel := make(chan ct.Con, rac.netfilterEventChannelBufferSize)
	defer close(eventChannel)

	log.Infof("Initializing file store...")
	if err := filestore.EnsureDirExists(rac.fileStoreDirectory); err != nil {
		return fmt.Errorf("could not create file store directory '%s': %w", rac.fileStoreDirectory, err)
	}
	store := filestore.NewFileStore(rac.fileStoreDirectory)
	if err := store.Load(); err != nil {
		return fmt.Errorf("error while loading existing file store content from directory '%s': %w", rac.fileStoreDirectory, err)
	}

	log.Infof("Initializing memory store...")
	memoryStore := memorystore.NewMemoryStore(store, rac.fileStoreChannelBufferSize, rac.fileStoreTraceStores, rac.netfilterTraceConnectionChange)
	memoryStore.StartStorageWorker()
	defer memoryStore.StopStorageWorker()

	log.Infof("Starting metrics server...")
	metricsServer := metrics.NewMetricsServer(store, memoryStore, rac.metricsPort, rac.metricsEnableErrorLog, rac.metricsEnableServiceMetrics, rac.metricsEnableByteMetrics, rac.metricsEnablePacketMetrics, rac.metricsEnableFlowCountMetrics, rac.metricsReportErrorsDuringCollection)
	go func() {
		metricsServer.ServiceMetrics()
	}()

	log.Infof("Connecting to netfilter sockets...")
	nfctDump, err := ct.Open(&ct.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to netfilter socket for dumping connection tracking table: %w", err)
	}
	defer nfctDump.Close()

	nfctEvent, err := ct.Open(&ct.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to netfilter socket for connection tracking events: %w", err)
	}
	defer nfctEvent.Close()

	log.Infof("Setting receive buffer of netfilter event socket to %d bytes...", rac.netfilterEventReceiveBufferSize)
	if err := nfctEvent.Con.SetReadBuffer(rac.netfilterEventReceiveBufferSize); err != nil {
		return fmt.Errorf("failed to set read buffer for netfilter event socket to %d: %w", rac.netfilterEventReceiveBufferSize, err)
	}

	if rac.netfilterIgnoreBufferErrors {
		log.Infof("Ignoring buffer related errors of netfilter event socket...")
		if err := nfctEvent.Con.SetOption(netlink.NoENOBUFS, true); err != nil {
			return fmt.Errorf("failed to disable buffer related errors (ENOBUFS) on netfilter event socket: %w", err)
		}
		if err := nfctEvent.Con.SetOption(netlink.BroadcastError, false); err != nil {
			return fmt.Errorf("failed to disable buffer related errors (BROADCAST_ERROR) on netfilter event socket: %w", err)
		}
	}

	log.Infof("Setting up ticker with period '%s'...", rac.netfilterDumpPeriod)
	ticker := time.NewTicker(rac.netfilterDumpPeriod)
	defer ticker.Stop()

	group := ct.NetlinkCtDestroy
	eventType := "close"
	if rac.netfilterCreateEventsEnabled {
		group |= ct.NetlinkCtNew
		eventType = "new|close"
	}
	log.Infof("Registering for netfilter connection tracking events (type %s) with buffer of size %d...", eventType, rac.netfilterEventChannelBufferSize)
	eventErrorChannel := nfctEvent.AttachErrChan()
	if err := nfctEvent.Register(context.Background(), ct.Conntrack, group, func(c ct.Con) int {
		eventChannel <- c
		return 0
	}); err != nil {
		return fmt.Errorf("failed to register for connection tracking events: %w", err)
	}

	log.Infof("Running network traffic measurement agent...")
	for {
		select {
		case e := <-eventErrorChannel:
			return fmt.Errorf("error during connection tracking event retrieval: %w", e)
		case c := <-eventChannel:
			rac.handleConnection(memoryStore, &c, log, "events", true, nil)
		case t := <-ticker.C:
			log.Infof("Dumping netfilter connection tracking table at '%s'...", t)
			for family := range []ct.Family{ct.IPv4, ct.IPv6} {
				table, err := nfctDump.Dump(ct.Conntrack, ct.Family(family))
				if err != nil {
					log.WithError(err).Warnf("Error during dumping connection tracking table: %v", err)
					continue
				}
				for _, c := range table {
					rac.handleConnection(memoryStore, &c, log, "dump", false, &t)
				}
			}
		}
	}
}

func (rac *runAgentCommand) handleConnection(memoryStore memorystore.MemoryStore, c *ct.Con, log *logrus.Entry, traceSource string, closed bool, time *time.Time) {
	if rac.netfilterIgnoreLoopbackTraffic && c.Origin.Src.IsLoopback() && c.Origin.Dst.IsLoopback() {
		return
	}
	utils.TraceConnection(rac.netfilterTraceConnections, log, c, traceSource)
	t := utils.GetTime(time, c)
	memoryStore.HandleConnection(c, traceSource, closed, t)
}
