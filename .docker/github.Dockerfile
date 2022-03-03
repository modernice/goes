FROM golang:1.18beta2
ARG TAGS
ENV TAGS $TAGS
WORKDIR /github
COPY go.mod go.sum /github/
RUN go mod download
COPY . .
CMD go test -v -tags=$TAGS ./...
