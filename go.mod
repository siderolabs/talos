module github.com/talos-systems/talos

go 1.13

replace (
	github.com/Azure/go-autorest v10.8.1+incompatible => github.com/Azure/go-autorest/autorest v0.9.1
	github.com/docker/distribution v2.7.1+incompatible => github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
	github.com/talos-systems/talos/pkg/machinery => ./pkg/machinery
)

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/hcsshim v0.8.10 // indirect
	github.com/Microsoft/hcsshim/test v0.0.0-20201124231931-de74fe8b94ae // indirect
	github.com/beevik/ntp v0.3.0
	github.com/containerd/cgroups v0.0.0-20201119153540-4cbc285b3327
	github.com/containerd/containerd v1.4.3
	github.com/containerd/continuity v0.0.0-20200928162600-f2cc35102c2a // indirect
	github.com/containerd/cri v1.19.0
	github.com/containerd/go-cni v1.0.1
	github.com/containerd/ttrpc v1.0.2 // indirect
	github.com/containerd/typeurl v1.0.1
	github.com/containernetworking/cni v0.8.1
	github.com/containernetworking/plugins v0.9.0
	github.com/coreos/go-iptables v0.4.5
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0
	github.com/dustin/go-humanize v1.0.0
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/emicklei/dot v0.15.0
	github.com/emicklei/go-restful v2.11.1+incompatible // indirect
	github.com/fatih/color v1.10.0
	github.com/firecracker-microvm/firecracker-go-sdk v0.22.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/fullsailor/pkcs7 v0.0.0-20190404230743-d7302db945fa
	github.com/gdamore/tcell/v2 v2.0.1-0.20201017141208-acf90d56d591
	github.com/gizak/termui/v3 v3.1.0
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/grpc-ecosystem/grpc-gateway v1.11.3 // indirect
	github.com/hashicorp/go-getter v1.5.1
	github.com/hashicorp/go-multierror v1.1.0
	github.com/insomniacslk/dhcp v0.0.0-20201112113307-4de412bc85d8
	github.com/jsimonetti/rtnetlink v0.0.0-20201125080424-8bebea019a6c
	github.com/mattn/go-isatty v0.0.12
	github.com/mdlayher/genetlink v1.0.0
	github.com/mdlayher/netlink v1.1.1
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v1.0.0-rc92 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20200728170252-4d89ac9fbff6
	github.com/pin/tftp v2.1.0+incompatible
	github.com/prometheus/procfs v0.2.0
	github.com/rivo/tview v0.0.0-20201118063654-f007e9ad3893
	github.com/rs/xid v1.2.1
	github.com/ryanuber/columnize v2.1.2+incompatible
	github.com/smira/go-xz v0.0.0-20201019130106-9921ed7a9935
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	github.com/talos-systems/crypto v0.2.1-0.20210202170911-39584f1b6e54
	github.com/talos-systems/go-blockdevice v0.2.1-0.20210216182145-8f976c203110
	github.com/talos-systems/go-cmd v0.0.0-20210216164758-68eb0067e0f0
	github.com/talos-systems/go-loadbalancer v0.1.0
	github.com/talos-systems/go-procfs v0.0.0-20210108152626-8cbc42d3dc24
	github.com/talos-systems/go-retry v0.2.0
	github.com/talos-systems/go-smbios v0.0.0-20200807005123-80196199691e
	github.com/talos-systems/grpc-proxy v0.2.0
	github.com/talos-systems/net v0.2.1-0.20210204205549-52c750994376
	github.com/talos-systems/os-runtime v0.0.0-20210216141502-28dd9aaf98d6
	github.com/talos-systems/talos/pkg/machinery v0.0.0-20200818212414-6a7cc0264819
	github.com/u-root/u-root v7.0.0+incompatible
	github.com/vmware-tanzu/sonobuoy v0.19.0
	github.com/vmware/vmw-guestinfo v0.0.0-20200218095840-687661b8bd8e
	go.etcd.io/etcd v0.5.0-alpha.5.0.20201125193152-8a03d2e9614b
	go.uber.org/zap v1.14.1 // indirect
	golang.org/x/crypto v0.0.0-20201124201722-c8d3bf9c5392 // indirect
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20201130171929-760e229fe7c5
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	golang.org/x/tools v0.0.0-20201217163546-c88dec5c6b05 // indirect
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20200609130330-bd2cb7843e1b
	google.golang.org/grpc v1.29.1
	google.golang.org/protobuf v1.25.0
	gopkg.in/freddierice/go-losetup.v1 v1.0.0-20170407175016-fc9adea44124
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	honnef.co/go/tools v0.0.1-2020.1.6 // indirect
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/apiserver v0.20.4 // indirect
	k8s.io/client-go v0.20.4
	k8s.io/cri-api v0.20.4
	k8s.io/kubelet v0.20.4
)
