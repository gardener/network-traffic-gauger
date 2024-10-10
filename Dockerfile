# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

############# builder
FROM golang:1.23.2 AS builder

WORKDIR /build
COPY . .
ARG TARGETARCH
RUN make release GOARCH=$TARGETARCH

############# network-traffic-gauger
FROM gcr.io/distroless/static-debian11 AS network-traffic-gauger

COPY --from=builder /build/net-gauger /net-gauger
ENTRYPOINT ["/net-gauger"]
