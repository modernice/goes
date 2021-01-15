FROM golang
WORKDIR /coverage
COPY go.mod go.sum /coverage/
RUN go mod download
COPY . .
CMD go test -v -race -tags=nats -covermode=atomic -coverprofile=out/coverage.out ./...
