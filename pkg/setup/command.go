// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	netfilterAccounting         = "/proc/sys/net/netfilter/nf_conntrack_acct"
	netfilterTimestamps         = "/proc/sys/net/netfilter/nf_conntrack_timestamp"
	netfilterDefaultPermissions = 0o644
)

type setupCommand struct {
	netfilterAccountingEnabled bool
	netfilterTimestampsEnabled bool
}

func CreateSetupCmd() *cobra.Command {
	sc := &setupCommand{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "setup host for network measurements",
		Long:  "setup host using netfilter accounting for measuring network traffic",
		RunE:  sc.setup,
	}
	cmd.Flags().BoolVar(&sc.netfilterAccountingEnabled, "enable-accounting", true, "enable connection tracking accounting in netfilter")
	cmd.Flags().BoolVar(&sc.netfilterTimestampsEnabled, "enable-timestamps", false, "enable connection tracking timestamps in netfilter")
	return cmd
}

func (sc *setupCommand) setup(_ *cobra.Command, _ []string) error {
	log := logrus.WithField("cmd", "setup")
	log.Infof("Checking and enabling netfilter accounting/timestamps (if required)...")

	if sc.netfilterAccountingEnabled {
		if err := checkAndEnableProcFsSetting(netfilterAccounting, "accounting"); err != nil {
			return err
		}
	}

	if sc.netfilterTimestampsEnabled {
		if err := checkAndEnableProcFsSetting(netfilterTimestamps, "timestamps"); err != nil {
			return err
		}
	}

	return nil
}

func checkAndEnableProcFsSetting(procFsFile string, topic string) error {
	log := logrus.WithField("cmd", "setup").WithField("option", topic)

	log.Infof("Checking netfilter %s setting in '%s'...", topic, procFsFile)
	if enabled, err := checkIfNetfilterOptionIsEnabled(procFsFile); err != nil {
		return err
	} else if enabled {
		log.Infof("Netfilter %s already enabled.", topic)
	} else {
		log.Infof("Netfilter %s not enabled, yet. Enabling...", topic)
		// '\n' is the delimiter used in proc file system for strings/vectors
		if err := os.WriteFile(procFsFile, []byte("1\n"), netfilterDefaultPermissions); err != nil {
			return fmt.Errorf("writing to file '%s' failed: %w", procFsFile, err)
		}
		log.Infof("Netfilter %s successfully enabled.", topic)
	}

	return nil
}

func CheckNetfilterPrerequisites() (bool, error) {
	return checkIfNetfilterOptionIsEnabled(netfilterAccounting)
}

func checkIfNetfilterOptionIsEnabled(procFsFile string) (bool, error) {
	data, err := os.ReadFile(procFsFile) // #nosec: G304 -- Only every called with two static paths. In reality files can be read from the Pod's file system only.
	if err != nil {
		return false, fmt.Errorf("failed to read '%s': %w", procFsFile, err)
	}
	if len(data) < 1 {
		return false, fmt.Errorf("'%s' has unexpected file length, expected at least one byte, but got %d", procFsFile, len(data))
	}
	return data[0] == '1', nil
}
