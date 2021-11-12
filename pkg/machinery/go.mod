module github.com/talos-systems/talos/pkg/machinery

go 1.17

// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
// which are then encoded as a valid YAML blocks with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20210315173758-8fb30b8e5a5b

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d
	github.com/containerd/go-cni v1.1.0
	github.com/cosi-project/runtime v0.0.0-20210906201716-5cb7f5002d77
	github.com/dustin/go-humanize v1.0.0
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jsimonetti/rtnetlink v0.0.0-20210922080037-435639c8e6a8
	github.com/mdlayher/ethtool v0.0.0-20210210192532-2b88debcdd43
	github.com/opencontainers/runtime-spec v1.0.3-0.20200929063507-e6143ca7d51d
	github.com/stretchr/testify v1.7.0
	github.com/talos-systems/crypto v0.3.4
	github.com/talos-systems/go-blockdevice v0.2.4
	github.com/talos-systems/go-debug v0.2.1
	github.com/talos-systems/net v0.3.1-0.20211112122313-0abe5bdae8f8
	golang.org/x/sys v0.0.0-20210927094055-39ccf1dd6fa6
	google.golang.org/genproto v0.0.0-20210924002016-3dee208752a0
	google.golang.org/grpc v1.41.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

require (
	github.com/containernetworking/cni v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/josharian/native v0.0.0-20200817173448-b6b71def0850 // indirect
	github.com/mdlayher/genetlink v1.0.0 // indirect
	github.com/mdlayher/netlink v1.4.1 // indirect
	github.com/mdlayher/socket v0.0.0-20210307095302-262dc9984e00 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5 // indirect
	golang.org/x/text v0.3.6 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
)
