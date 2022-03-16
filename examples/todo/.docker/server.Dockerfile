FROM golang:1.18 AS build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
WORKDIR /build/examples/todo
RUN CGO_ENABLED=0 go build -tags timetzdata -o ./server ./cmd/server

FROM alpine

COPY --from=build /build/examples/todo/server /server

ENTRYPOINT ["/server"]
