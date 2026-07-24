module github.com/siderolabs/talos/pkg/machinery

go 1.26.5

// forked ethtool introduces missing APIs
replace github.com/mdlayher/ethtool => github.com/siderolabs/ethtool v0.6.0-sidero

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/containerd/go-cni v1.1.13
	github.com/cosi-project/runtime v1.16.2
	github.com/dustin/go-humanize v1.0.1
	github.com/emicklei/dot v1.11.0
	github.com/evanphx/json-patch v5.9.11+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/google/cel-go v0.29.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jsimonetti/rtnetlink/v2 v2.2.1-0.20260714114318-c87a4183a51a
	github.com/mdlayher/ethtool v0.6.1
	github.com/neticdk/go-stdlib v1.0.1
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/planetscale/vtprotobuf v0.6.1-0.20260702190614-8ae5a48058df
	github.com/ryanuber/go-glob v1.0.0
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2
	github.com/siderolabs/crypto v0.6.5
	github.com/siderolabs/gen v0.8.7
	github.com/siderolabs/go-api-signature v0.3.13
	github.com/siderolabs/go-pointer v1.0.1
	github.com/siderolabs/net v0.4.0
	github.com/siderolabs/protoenc v0.2.4
	github.com/stretchr/testify v1.11.1
	go.uber.org/zap v1.28.0
	go.yaml.in/yaml/v4 v4.0.0-rc.6
	golang.org/x/net v0.57.0
	google.golang.org/genproto/googleapis/api v0.0.0-20260723215102-3fe39f3c1018
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260723215102-3fe39f3c1018
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af
)

require (
	cel.dev/expr v0.25.2 // indirect
	github.com/ProtonMail/go-crypto v1.4.1 // indirect
	github.com/ProtonMail/gopenpgp/v3 v3.4.1 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cloudflare/circl v1.6.4 // indirect
	github.com/containernetworking/cni v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/gertd/go-pluralize v0.2.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/mdlayher/genetlink v1.4.0 // indirect
	github.com/mdlayher/netlink v1.11.2 // indirect
	github.com/mdlayher/socket v0.6.1 // indirect
	github.com/petermattis/goid v0.0.0-20260713124913-97594f28f5ca // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.9 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/exp v0.0.0-20260709172345-9ea1abe57597 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
