package proto

//go:generate protoc -I . --go_out=module=github.com/modernice/goes/cli/internal/proto:. --go-grpc_out=module=github.com/modernice/goes/cli/internal/proto:. projection.proto
