FROM golang
ARG SERVICE
WORKDIR /test
COPY go.mod go.sum /test/
RUN go mod download
COPY . .
CMD go test -v -race -tags=integration ./...
