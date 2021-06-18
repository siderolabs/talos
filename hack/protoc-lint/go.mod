module github.com/talos-systems/talos-hack-protoc-lint

go 1.16

replace github.com/talos-systems/talos/pkg/machinery => ../../pkg/machinery

require (
	github.com/stretchr/testify v1.7.0
	github.com/talos-systems/talos/pkg/machinery v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.26.0
)
