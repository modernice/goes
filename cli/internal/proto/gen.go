package proto

//go:generate protoc -I . --go_out=plugins=grpc,paths=source_relative:. projection.proto
