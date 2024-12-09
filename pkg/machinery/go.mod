module github.com/siderolabs/talos/pkg/machinery

go 1.23.3

// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
// which are then encoded as a valid YAML blocks with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml/v2 v2.4.0

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/containerd/go-cni v1.1.10
	github.com/cosi-project/runtime v0.7.2
	github.com/dustin/go-humanize v1.0.1
	github.com/emicklei/dot v1.6.3
	github.com/evanphx/json-patch v5.9.0+incompatible
	github.com/fatih/color v1.18.0
	github.com/ghodss/yaml v1.0.0
	github.com/google/cel-go v0.22.1
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hexops/gotextdiff v1.0.3
	github.com/jsimonetti/rtnetlink/v2 v2.0.2
	github.com/mdlayher/ethtool v0.2.0
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/planetscale/vtprotobuf v0.6.1-0.20241121165744-79df5c4772f2
	github.com/ryanuber/go-glob v1.0.0
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.1
	github.com/siderolabs/crypto v0.5.0
	github.com/siderolabs/gen v0.7.0
	github.com/siderolabs/go-api-signature v0.3.6
	github.com/siderolabs/go-blockdevice/v2 v2.0.6
	github.com/siderolabs/go-pointer v1.0.0
	github.com/siderolabs/net v0.4.0
	github.com/siderolabs/protoenc v0.2.1
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.27.0
	google.golang.org/genproto/googleapis/api v0.0.0-20241118233622-e639e219e697
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241118233622-e639e219e697
	google.golang.org/grpc v1.68.0
	google.golang.org/protobuf v1.35.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cel.dev/expr v0.18.0 // indirect
	github.com/ProtonMail/go-crypto v1.1.3 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.8.1 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cloudflare/circl v1.5.0 // indirect
	github.com/containernetworking/cni v1.2.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/gertd/go-pluralize v0.2.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.24.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.29.0 // indirect
	golang.org/x/exp v0.0.0-20241108190413-2d47ceb2692f // indirect
	golang.org/x/net v0.31.0 // indirect
	golang.org/x/sync v0.9.0 // indirect
	golang.org/x/sys v0.27.0 // indirect
	golang.org/x/text v0.20.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
