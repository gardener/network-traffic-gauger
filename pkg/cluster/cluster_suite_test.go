// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestClusterInfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Info Suite")
}
