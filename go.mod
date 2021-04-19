module github.com/talos-systems/talos

go 1.16

replace (
	github.com/talos-systems/talos/pkg/machinery => ./pkg/machinery
	// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
	// which are then encoded as a valid YAML blocks with proper indentiation
	gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20210315173758-8fb30b8e5a5b
)

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/hcsshim v0.8.10 // indirect
	github.com/Microsoft/hcsshim/test v0.0.0-20201124231931-de74fe8b94ae // indirect
	github.com/beevik/ntp v0.3.0
	github.com/containerd/cgroups v0.0.0-20201119153540-4cbc285b3327
	github.com/containerd/containerd v1.4.4
	github.com/containerd/continuity v0.0.0-20200928162600-f2cc35102c2a // indirect
	github.com/containerd/cri v1.19.0
	github.com/containerd/go-cni v1.0.2
	github.com/containerd/ttrpc v1.0.2 // indirect
	github.com/containerd/typeurl v1.0.1
	github.com/containernetworking/cni v0.8.1
	github.com/containernetworking/plugins v0.9.1
	github.com/coreos/go-iptables v0.5.0
	github.com/coreos/go-semver v0.3.0
	github.com/cosi-project/runtime v0.0.0-20210409233936-10d6103c19ab
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.4+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/dustin/go-humanize v1.0.0
	github.com/elazarl/goproxy v0.0.0-20210110162100-a92cc753f88e // indirect
	github.com/emicklei/dot v0.15.0
	github.com/emicklei/go-restful v2.15.0+incompatible // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/fatih/color v1.10.0
	github.com/firecracker-microvm/firecracker-go-sdk v0.22.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/fullsailor/pkcs7 v0.0.0-20190404230743-d7302db945fa
	github.com/gdamore/tcell/v2 v2.2.0
	github.com/gizak/termui/v3 v3.1.0
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.5
	github.com/google/uuid v1.2.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/hashicorp/go-getter v1.5.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/insomniacslk/dhcp v0.0.0-20210120172423-cc9239ac6294
	github.com/jsimonetti/rtnetlink v0.0.0-20210226120601-1b79e63a70a0
	github.com/mattn/go-isatty v0.0.12
	github.com/mdlayher/genetlink v1.0.0
	github.com/mdlayher/netlink v1.4.0
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v1.0.0-rc92 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20200728170252-4d89ac9fbff6
	github.com/pin/tftp v2.1.0+incompatible
	github.com/plunder-app/kube-vip v0.3.2
	github.com/prometheus/procfs v0.6.0
	github.com/rivo/tview v0.0.0-20210217110421-8a8f78a6dd01
	github.com/rs/xid v1.2.1
	github.com/ryanuber/columnize v2.1.2+incompatible
	github.com/smira/go-xz v0.0.0-20201019130106-9921ed7a9935
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0
	github.com/talos-systems/crypto v0.2.1-0.20210202170911-39584f1b6e54
	github.com/talos-systems/go-blockdevice v0.2.1-0.20210407132431-1d830a25f64f
	github.com/talos-systems/go-cmd v0.0.0-20210216164758-68eb0067e0f0
	github.com/talos-systems/go-loadbalancer v0.1.0
	github.com/talos-systems/go-procfs v0.0.0-20210108152626-8cbc42d3dc24
	github.com/talos-systems/go-retry v0.2.1-0.20210119124456-b9dc1a990133
	github.com/talos-systems/go-smbios v0.0.0-20201228201610-fb425d4727e6
	github.com/talos-systems/grpc-proxy v0.2.0
	github.com/talos-systems/net v0.2.1-0.20210212213224-05190541b0fa
	github.com/talos-systems/talos/pkg/machinery v0.0.0-20210302191918-8ffb55943c71
	github.com/u-root/u-root v7.0.0+incompatible
	github.com/vmware-tanzu/sonobuoy v0.20.0
	github.com/vmware/govmomi v0.24.0
	github.com/vmware/vmw-guestinfo v0.0.0-20200218095840-687661b8bd8e
	go.etcd.io/etcd/api/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/client/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/etcdctl/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/pkg/v3 v3.5.0-alpha.0
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210301091718-77cc2087c03b
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20200609130330-bd2cb7843e1b
	google.golang.org/grpc v1.37.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/freddierice/go-losetup.v1 v1.0.0-20170407175016-fc9adea44124
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	honnef.co/go/tools v0.1.2 // indirect
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/apiserver v0.21.0 // indirect
	k8s.io/client-go v0.21.0
	k8s.io/cri-api v0.21.0
	k8s.io/kubectl v0.21.0
	k8s.io/kubelet v0.21.0
)
