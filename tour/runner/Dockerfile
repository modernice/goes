FROM golang:1.24.1-bookworm

WORKDIR /opt/goes

COPY go.mod go.sum ./
RUN go mod download

ENV GOCACHE=/tmp/go-cache

WORKDIR /workspace
