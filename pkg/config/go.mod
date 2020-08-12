module github.com/talos-systems/talos/pkg/config

go 1.14

replace github.com/talos-systems/talos/api => ../../api

replace github.com/talos-systems/talos/pkg/client => ../client

replace github.com/talos-systems/talos/pkg/constants => ../constants

replace github.com/talos-systems/talos/pkg/crypto => ../crypto

replace github.com/talos-systems/talos/pkg/grpc => ../grpc

replace github.com/talos-systems/talos/pkg/net => ../net

require (
	github.com/hashicorp/go-multierror v1.1.0
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/stretchr/testify v1.6.1
	github.com/talos-systems/bootkube-plugin v0.0.0-20200729203641-12d463a3e54e
	github.com/talos-systems/talos/pkg/client v0.0.0-00010101000000-000000000000
	github.com/talos-systems/talos/pkg/constants v0.0.0-00010101000000-000000000000
	github.com/talos-systems/talos/pkg/crypto v0.0.0-00010101000000-000000000000
	github.com/talos-systems/talos/pkg/net v0.0.0-00010101000000-000000000000
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
)
