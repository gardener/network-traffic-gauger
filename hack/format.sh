#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Format"

${GOIMPORTS:-goimports} -l -w $@

