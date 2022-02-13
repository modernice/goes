FROM golang:1.18beta2
WORKDIR /test
COPY go.mod go.sum /test/
RUN go mod download
COPY . .
CMD go test -bench=. -tags=nats ./command/cmdbus
