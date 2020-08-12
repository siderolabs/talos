module github.com/talos-systems/talos/pkg/client

go 1.14

replace (
	github.com/talos-systems/talos/api => ../../api
	github.com/talos-systems/talos/pkg/constants => ../constants
	github.com/talos-systems/talos/pkg/crypto => ../crypto
	github.com/talos-systems/talos/pkg/grpc => ../grpc
	github.com/talos-systems/talos/pkg/net => ../net
)

require (
	github.com/golang/protobuf v1.4.2
	github.com/hashicorp/go-multierror v1.1.0
	github.com/stretchr/testify v1.6.1
	github.com/talos-systems/talos/api v0.0.0-00010101000000-000000000000
	github.com/talos-systems/talos/pkg/constants v0.0.0-00010101000000-000000000000
	github.com/talos-systems/talos/pkg/grpc v0.0.0-00010101000000-000000000000
	github.com/talos-systems/talos/pkg/net v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.31.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
)
