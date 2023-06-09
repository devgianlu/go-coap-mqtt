FROM golang:1.20-alpine as builder

WORKDIR /usr/src
COPY . .
RUN go mod download && go build -o ./gateway .

FROM alpine

COPY --from=builder /usr/src/gateway /usr/bin/gateway

ENTRYPOINT ["/usr/bin/gateway"]