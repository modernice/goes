FROM golang

WORKDIR /goes
COPY go.mod go.sum /goes/
RUN go mod download
COPY . .

WORKDIR /goes/examples/ecommerce
RUN go mod download
RUN go build -o /cli/cli ./cmd/cli

WORKDIR /cli
