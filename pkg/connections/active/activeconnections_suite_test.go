// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package active

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestActiveConnections(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ActiveConnections Suite")
}
