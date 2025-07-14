FROM golang:1.23.0
ARG TAGS
ENV TAGS $TAGS
WORKDIR /github
COPY go.mod go.sum /github/
RUN go mod download
COPY . .
RUN go install gotest.tools/gotestsum@latest
CMD gotestsum --packages=./... -- -v -tags=$TAGS
