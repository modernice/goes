FROM golang:1.22.9
ARG TAGS
ENV TAGS $TAGS
WORKDIR /coverage
COPY go.mod go.sum /coverage/
RUN go mod download
COPY . .
CMD go test -v -race -tags=$TAGS -covermode=atomic -coverprofile=out/coverage.out ./...
