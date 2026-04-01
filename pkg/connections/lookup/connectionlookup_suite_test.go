// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lookup

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConnectionLookup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ConnectionLookup Suite")
}
