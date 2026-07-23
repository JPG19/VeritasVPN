FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /src
COPY lib/ lib/
COPY go.work go.work
COPY services/veritas-agent/ services/veritas-agent/
WORKDIR /src/services/veritas-agent
RUN CGO_ENABLED=0 go build -o /usr/local/bin/veritas-agent ./cmd/agent/

FROM alpine:3.19
RUN apk add --no-cache wireguard-tools nftables
COPY --from=builder /usr/local/bin/veritas-agent /usr/local/bin/veritas-agent
CMD ["/usr/local/bin/veritas-agent"]
