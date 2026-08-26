// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package lookup

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConnectionLookup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ConnectionLookup Suite")
}
