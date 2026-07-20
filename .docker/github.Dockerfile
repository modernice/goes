FROM golang:1.25.0
ARG TAGS
ENV TAGS $TAGS
WORKDIR /github
COPY go.mod go.sum /github/
RUN go mod download
COPY . .
RUN go install gotest.tools/gotestsum@latest
CMD sh -c 'gotestsum --raw-command -- go test -json -v -tags="$TAGS" $(bash ./scripts/test-packages ./...)'
