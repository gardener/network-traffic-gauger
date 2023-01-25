// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package filestore

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFileStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FileStore Suite")
}
