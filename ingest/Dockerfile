FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 go build -o /fleet-ingest .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /fleet-ingest /app/fleet-ingest

EXPOSE 8080

ENTRYPOINT ["/app/fleet-ingest"]
