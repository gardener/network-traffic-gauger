# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

############# builder
FROM golang:1.23.2 AS builder

WORKDIR /build

# Copy go mod and sum files
COPY go.mod go.sum ./
# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

COPY . .
ARG TARGETARCH
RUN make release GOARCH=$TARGETARCH

############# network-traffic-gauger
FROM gcr.io/distroless/static-debian11 AS network-traffic-gauger

COPY --from=builder /build/net-gauger /net-gauger
ENTRYPOINT ["/net-gauger"]
