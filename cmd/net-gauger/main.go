// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/gardener/network-traffic-gauger/pkg/agent"
	"github.com/gardener/network-traffic-gauger/pkg/setup"

	"github.com/spf13/cobra"
)

var (
	// Version is injected by build.
	Version string
	// ImageTag is injected by build.
	ImageTag string

	rootCmd = &cobra.Command{
		Use:   "net-gauger",
		Short: "Network traffic gauger (" + Version + ")",
	}
)

func main() {
	rootCmd.AddCommand(agent.CreateRunAgentCmd())
	rootCmd.AddCommand(setup.CreateSetupCmd())
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
