FROM golang:1.18
ARG TAGS
ENV TAGS $TAGS
WORKDIR /github
COPY go.mod go.sum /github/
RUN go mod download
COPY . .
RUN go install gotest.tools/gotestsum@latest
CMD gotestsum --rerun-fails=3 --packages=./... -- -v -tags=$TAGS
