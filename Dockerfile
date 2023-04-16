FROM golang:1.20-buster as builder
ENV GRPC_HEALTH_PROBE_VERSION="v0.4.14"

RUN curl -Lso /envoy-preflight "https://github.com/monzo/envoy-preflight/releases/download/v1.0/envoy-preflight" && \
    chmod +x /envoy-preflight && \
    curl -Lso /grpc_health_probe "https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64" && \
    chmod +x /grpc_health_probe
WORKDIR /go/src/infrabin-connect
COPY . /go/src/infrabin-connect
RUN make generate && \
    make build

FROM gcr.io/distroless/base-debian11@sha256:9283685c6be8f12cec61d9b6812ed71a6ca9c8cebe211c8df7dbc4d1194591bb
COPY --from=builder /go/src/infrabin-connect/infrabin-connect /usr/local/bin/infrabin-connect
COPY --from=builder /envoy-preflight /envoy-preflight
ENTRYPOINT [ "/envoy-preflight", "/usr/local/bin/infrabin-connect" ]
