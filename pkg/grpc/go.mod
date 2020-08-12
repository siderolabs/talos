module github.com/talos-systems/talos/pkg/grpc

go 1.14

replace (
	github.com/talos-systems/talos/api => ../../api
	github.com/talos-systems/talos/pkg/constants => ../constants
	github.com/talos-systems/talos/pkg/crypto => ../crypto
	github.com/talos-systems/talos/pkg/net => ../net
)

require (
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/hashicorp/go-multierror v1.1.0
	github.com/stretchr/testify v1.6.1
	github.com/talos-systems/grpc-proxy v0.2.0
	github.com/talos-systems/talos/api v0.0.0-00010101000000-000000000000
	github.com/talos-systems/talos/pkg/constants v0.0.0-00010101000000-000000000000
	github.com/talos-systems/talos/pkg/crypto v0.0.0-00010101000000-000000000000
	github.com/talos-systems/talos/pkg/net v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.29.0
)
