# syntax=docker/dockerfile:1
FROM golang:1.21 AS build
WORKDIR /app
COPY ./src ./src
WORKDIR /app/src
RUN go mod download || true
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/cprotocol ./cmd/cprotocol

FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=build /out/cprotocol /usr/local/bin/cprotocol
COPY ./config /etc/cprotocol/config
ENTRYPOINT ["/usr/local/bin/cprotocol", "monitor", "--exchange", "kraken"]
