FROM golang
ARG SERVICE
ENV SERVICE $SERVICE

WORKDIR /goes
COPY go.mod go.sum /goes/
RUN go mod download
COPY . .

WORKDIR /goes/examples/ecommerce
RUN go mod download
RUN go build -o /$SERVICE/service ./cmd/$SERVICE

WORKDIR /$SERVICE

ENTRYPOINT /${SERVICE}/service
