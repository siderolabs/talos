module github.com/siderolabs/talos/pkg/machinery

go 1.24.0

replace (
	// forked ethtool introduces missing APIs
	github.com/mdlayher/ethtool => github.com/siderolabs/ethtool v0.4.0-sidero

	// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
	// which are then encoded as a valid YAML blocks with proper indentiation
	gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20220527175918-f17b0f05cf2c
)

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/containerd/go-cni v1.1.12
	github.com/cosi-project/runtime v0.10.6
	github.com/dustin/go-humanize v1.0.1
	github.com/emicklei/dot v1.8.0
	github.com/evanphx/json-patch v5.9.11+incompatible
	github.com/fatih/color v1.18.0
	github.com/ghodss/yaml v1.0.0
	github.com/google/cel-go v0.25.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hexops/gotextdiff v1.0.3
	github.com/jsimonetti/rtnetlink/v2 v2.0.3
	github.com/mdlayher/ethtool v0.4.0
	github.com/opencontainers/runtime-spec v1.2.1
	github.com/planetscale/vtprotobuf v0.6.1-0.20241121165744-79df5c4772f2
	github.com/ryanuber/go-glob v1.0.0
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/siderolabs/crypto v0.6.0
	github.com/siderolabs/gen v0.8.3
	github.com/siderolabs/go-api-signature v0.3.6
	github.com/siderolabs/go-pointer v1.0.1
	github.com/siderolabs/net v0.4.0
	github.com/siderolabs/protoenc v0.2.2
	github.com/stretchr/testify v1.10.0
	google.golang.org/genproto/googleapis/api v0.0.0-20250603155806-513f23925822
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822
	google.golang.org/grpc v1.72.2
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cel.dev/expr v0.23.1 // indirect
	github.com/ProtonMail/go-crypto v1.2.0 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.8.3 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/containernetworking/cni v1.2.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/gertd/go-pluralize v0.2.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/petermattis/goid v0.0.0-20240813172612-4fcff4a6cae7 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.5 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/exp v0.0.0-20250128182459-e0ece0dbea4c // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
