module github.com/talos-systems/talos

go 1.13

replace (
	github.com/Azure/go-autorest v10.8.1+incompatible => github.com/Azure/go-autorest/autorest v0.9.1
	github.com/docker/distribution v2.7.1+incompatible => github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
	github.com/talos-systems/talos/pkg/machinery => ./pkg/machinery
)

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/hcsshim v0.8.9 // indirect
	github.com/Microsoft/hcsshim/test v0.0.0-20200831205110-d2cba219a8d7 // indirect
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2
	github.com/beevik/ntp v0.3.0
	github.com/containerd/cgroups v0.0.0-20200710171044-318312a37340
	github.com/containerd/containerd v1.4.0
	github.com/containerd/continuity v0.0.0-20200710164510-efbc4488d8fe // indirect
	github.com/containerd/cri v1.11.1
	github.com/containerd/go-cni v1.0.0
	github.com/containerd/ttrpc v1.0.1 // indirect
	github.com/containerd/typeurl v1.0.1
	github.com/containernetworking/cni v0.8.0
	github.com/containernetworking/plugins v0.8.6
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0
	github.com/dustin/go-humanize v1.0.0
	github.com/firecracker-microvm/firecracker-go-sdk v0.21.0
	github.com/fullsailor/pkcs7 v0.0.0-20190404230743-d7302db945fa
	github.com/gizak/termui/v3 v3.1.0
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/hashicorp/go-getter v1.4.1
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hugelgupf/socketpair v0.0.0-20190730060125-05d35a94e714 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20200711001733-e1b69ee5fb33
	github.com/jsimonetti/rtnetlink v0.0.0-20200709124027-1aae10735293
	github.com/kubernetes-sigs/bootkube v0.14.1-0.20200817205730-0b4482256ca1
	github.com/mdlayher/ethernet v0.0.0-20190606142754-0394541c37b7 // indirect
	github.com/mdlayher/genetlink v1.0.0
	github.com/mdlayher/netlink v1.1.0
	github.com/mdlayher/raw v0.0.0-20191009151244-50f2db8cc065 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v1.0.0-rc92 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20200728170252-4d89ac9fbff6
	github.com/particledecay/kconf v1.8.0
	github.com/pin/tftp v2.1.0+incompatible
	github.com/prometheus/procfs v0.1.3
	github.com/rs/xid v1.2.1
	github.com/ryanuber/columnize v2.1.0+incompatible
	github.com/smira/go-xz v0.0.0-20150414201226-0c531f070014
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.6.1
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	github.com/talos-systems/bootkube-plugin v0.0.0-20200915135634-229d57e818f3
	github.com/talos-systems/crypto v0.2.0
	github.com/talos-systems/go-loadbalancer v0.1.0
	github.com/talos-systems/go-procfs v0.0.0-20200219015357-57c7311fdd45
	github.com/talos-systems/go-retry v0.1.1-0.20200922131245-752f081252cf
	github.com/talos-systems/go-smbios v0.0.0-20200219201045-94b8c4e489ee
	github.com/talos-systems/grpc-proxy v0.2.0
	github.com/talos-systems/net v0.2.0
	github.com/talos-systems/talos/pkg/machinery v0.0.0-20200818212414-6a7cc0264819
	github.com/u-root/u-root v6.0.0+incompatible // indirect
	github.com/vishvananda/netns v0.0.0-20200520041808-52d707b772fe // indirect
	github.com/vmware-tanzu/sonobuoy v0.19.0
	github.com/vmware/vmw-guestinfo v0.0.0-20200218095840-687661b8bd8e
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200819165624-17cef6e3e9d5 // v3.4.10
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	golang.org/x/sys v0.0.0-20200728102440-3e129f6d46b1
	golang.org/x/text v0.3.3
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	golang.org/x/tools v0.0.0-20200716134326-a8f9df4c9543 // indirect
	google.golang.org/grpc v1.29.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/freddierice/go-losetup.v1 v1.0.0-20170407175016-fc9adea44124
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/api v0.19.1
	k8s.io/apimachinery v0.19.1
	k8s.io/apiserver v0.19.1
	k8s.io/client-go v0.19.1
	k8s.io/cri-api v0.19.1
	k8s.io/kubelet v0.19.1
)
