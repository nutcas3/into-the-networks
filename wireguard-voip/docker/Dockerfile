FROM golang:1.26.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o wireguard-voip ./cmd

FROM alpine:latest

RUN apk --no-cache add ca-certificates wireguard-tools iproute2

WORKDIR /root/

COPY --from=builder /app/wireguard-voip .
COPY --from=builder /app/web ./web

EXPOSE 8085
EXPOSE 51820/udp

CMD ["./wireguard-voip"]
