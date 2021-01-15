FROM golang:alpine
WORKDIR /coverage
COPY go.mod go.sum /coverage/
RUN go mod download
COPY . .
CMD CGO_ENABLED=0 go test -v -tags=nats -covermode=count -coverprofile=out/coverage.out ./...
