FROM golang:1.22.9
WORKDIR /test
COPY go.mod go.sum /test/
RUN go mod download
COPY . .
CMD go test -bench=. -tags=nats ./command/cmdbus
