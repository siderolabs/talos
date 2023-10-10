module github.com/siderolabs/talos/pkg/machinery

go 1.20

// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
// which are then encoded as a valid YAML blocks with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20220527175918-f17b0f05cf2c

require (
	github.com/containerd/go-cni v1.1.9
	github.com/cosi-project/runtime v0.3.1
	github.com/dustin/go-humanize v1.0.1
	github.com/evanphx/json-patch v5.6.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-version v1.6.0
	github.com/jsimonetti/rtnetlink v1.3.4
	github.com/mdlayher/ethtool v0.1.0
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8
	github.com/siderolabs/crypto v0.4.1
	github.com/siderolabs/gen v0.4.5
	github.com/siderolabs/go-api-signature v0.2.6
	github.com/siderolabs/go-blockdevice v0.4.6
	github.com/siderolabs/go-pointer v1.0.0
	github.com/siderolabs/net v0.4.0
	github.com/siderolabs/protoenc v0.2.0
	github.com/stretchr/testify v1.8.4
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230726155614-23370e0ffb3e
	google.golang.org/grpc v1.57.1
	google.golang.org/protobuf v1.31.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ProtonMail/go-crypto v0.0.0-20230717121422-5aa5874ade95 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.7.2 // indirect
	github.com/adrg/xdg v0.4.0 // indirect
	github.com/cloudflare/circl v1.3.3 // indirect
	github.com/containernetworking/cni v1.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gertd/go-pluralize v0.2.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.4.1 // indirect
	github.com/onsi/ginkgo/v2 v2.2.0 // indirect
	github.com/onsi/gomega v1.20.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/crypto v0.11.0 // indirect
	golang.org/x/net v0.12.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.10.0 // indirect
	golang.org/x/text v0.11.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230726155614-23370e0ffb3e // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
