// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
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
	RunSpecs(t, "ClusterInfo Suite")
}
