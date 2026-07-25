FROM golang:1.25.4-alpine3.22@sha256:d3f0cf7723f3429e3f9ed846243970b20a2de7bae6a5b66fc5914e228d831bbb AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/rc-car-server \
    ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/telemetry-relay \
    ./cmd/telemetry-relay

FROM alpine:3.22.2@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/rc-car-server /app/rc-car-server
COPY --from=build /out/telemetry-relay /app/telemetry-relay
COPY web /app/web

USER 65532:65532

EXPOSE 8081/tcp
EXPOSE 4211/udp

ENTRYPOINT ["/app/rc-car-server"]
