# ============= Compilation Stage ================
FROM golang:1.22.10-bookworm AS builder

WORKDIR /build
# Copy and download avalanche dependencies using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download
# Copy the code into the container
COPY . .
# Build avalanchego
RUN ./scripts/build.sh

# ============= Cleanup Stage ================
FROM debian:12-slim
WORKDIR /

# Copy the executables into the container
COPY --from=builder /build/bin/avalanche .
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
RUN /avalanche config update disable
ENTRYPOINT [ "./avalanche" ]
