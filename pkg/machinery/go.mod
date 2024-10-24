module github.com/siderolabs/talos/pkg/machinery

go 1.23.2

// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
// which are then encoded as a valid YAML blocks with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20220527175918-f17b0f05cf2c

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/containerd/go-cni v1.1.10
	github.com/cosi-project/runtime v0.6.4
	github.com/dustin/go-humanize v1.0.1
	github.com/emicklei/dot v1.6.2
	github.com/evanphx/json-patch v5.9.0+incompatible
	github.com/fatih/color v1.18.0
	github.com/ghodss/yaml v1.0.0
	github.com/google/cel-go v0.21.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hexops/gotextdiff v1.0.3
	github.com/jsimonetti/rtnetlink/v2 v2.0.2
	github.com/mdlayher/ethtool v0.2.0
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/planetscale/vtprotobuf v0.6.1-0.20241011083415-71c992bc3c87
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/siderolabs/crypto v0.5.0
	github.com/siderolabs/gen v0.6.1
	github.com/siderolabs/go-api-signature v0.3.6
	github.com/siderolabs/go-blockdevice v0.4.8
	github.com/siderolabs/go-blockdevice/v2 v2.0.3
	github.com/siderolabs/go-pointer v1.0.0
	github.com/siderolabs/net v0.4.0
	github.com/siderolabs/protoenc v0.2.1
	github.com/stretchr/testify v1.9.0
	go.uber.org/zap v1.27.0
	google.golang.org/genproto/googleapis/api v0.0.0-20241021214115-324edc3d5d38
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241021214115-324edc3d5d38
	google.golang.org/grpc v1.67.1 // do not update to 1.68.0 until we find a way around https://github.com/grpc/grpc-go/pull/7535
	google.golang.org/protobuf v1.35.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ProtonMail/go-crypto v1.1.0-alpha.5.0.20240827111422-b5837fa4476e // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.7.5 // indirect
	github.com/adrg/xdg v0.5.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/cloudflare/circl v1.3.9 // indirect
	github.com/containernetworking/cni v1.2.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/gertd/go-pluralize v0.2.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
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
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
