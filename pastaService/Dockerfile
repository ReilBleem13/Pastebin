FROM golang:1.24

RUN apt-get update && apt-get install -y \
    librdkafka-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/app ./cmd/app/main.go

COPY config.yml /app/config.yml

EXPOSE 10002

CMD ["/app/app"]
