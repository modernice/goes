FROM golang:1.18
ARG TAGS
ARG TEST_PATH=./...
ENV TAGS $TAGS
ENV TEST_PATH $TEST_PATH
WORKDIR /test
COPY go.mod go.sum /test/
RUN go mod download
COPY . .
CMD go test -v -tags=$TAGS $TEST_PATH
