FROM golang
ARG TAG
WORKDIR /test
COPY go.mod go.sum /test/
RUN go mod download
COPY . .
CMD go test -v -race -tags=${TAG} ./...
