FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -o /app ./cmd/main.go

FROM alpine:latest AS final

WORKDIR /app
COPY --from=builder /app /app/app
COPY config.yml /app/config.yml

EXPOSE 10001
CMD ["/app/app"]
