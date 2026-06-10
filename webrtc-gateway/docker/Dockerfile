FROM golang:1.26.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o webrtc-gateway ./cmd

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/webrtc-gateway .
COPY web ./web

EXPOSE 8083

CMD ["./webrtc-gateway"]
