FROM golang:alpine
WORKDIR /nats
COPY go.mod go.sum /nats/
RUN go mod download
COPY . .
CMD CGO_ENABLED=0 go test -v --tags=nats ./...
