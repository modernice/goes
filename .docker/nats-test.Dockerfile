FROM golang
WORKDIR /nats
COPY go.mod go.sum /nats/
RUN go mod download
COPY . .
CMD go test -v -race -tags=nats ./...
