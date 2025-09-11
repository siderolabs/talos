module github.com/siderolabs/talos

go 1.25.0

replace (
	// see e.g. https://github.com/grpc/grpc-go/issues/6696
	cloud.google.com/go => cloud.google.com/go v0.100.2

	// forked coredns so we don't carry caddy and other stuff into the Talos
	github.com/coredns/coredns => github.com/siderolabs/coredns v1.12.54

	// forked ethtool introduces missing APIs
	github.com/mdlayher/ethtool => github.com/siderolabs/ethtool v0.4.0-sidero

	// see https://github.com/mdlayher/kobject/pull/5
	github.com/mdlayher/kobject => github.com/smira/kobject v0.0.0-20240304111826-49c8d4613389

	// Use nested module.
	github.com/siderolabs/talos/pkg/machinery => ./pkg/machinery

	// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
	// which are then encoded as a valid YAML blocks with proper indentiation
	gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20220527175918-f17b0f05cf2c
)

// fd-leak related replacements: https://github.com/siderolabs/talos/issues/9412
// https://github.com/insomniacslk/dhcp/pull/550
replace github.com/insomniacslk/dhcp => github.com/smira/dhcp v0.0.0-20250407153013-99942baa5d59

// deadcode elimination fix replacement: https://github.com/siderolabs/talos/issues/11296
// upstream PR: https://github.com/containerd/containerd/pull/12175
// this a fork with containerd 2.1 branch + the commit from the PR above
replace github.com/containerd/containerd/v2 => github.com/smira/containerd/v2 v2.0.0-20250806103510-dcf2fc86e156

// Kubernetes dependencies sharing the same version.
require (
	k8s.io/api v0.34.1
	k8s.io/apiextensions-apiserver v0.34.1
	k8s.io/apimachinery v0.34.1
	k8s.io/apiserver v0.34.1
	k8s.io/client-go v0.34.1
	k8s.io/component-base v0.34.1
	k8s.io/cri-api v0.34.1
	k8s.io/kube-scheduler v0.34.1
	k8s.io/kubectl v0.34.1
	k8s.io/kubelet v0.34.1
	k8s.io/pod-security-admission v0.34.1
)

require (
	cloud.google.com/go/compute/metadata v0.8.0
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.19.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.11.0
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates v1.4.0
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys v1.4.0
	github.com/alexflint/go-filemutex v1.3.0
	github.com/aws/aws-sdk-go-v2/config v1.31.6
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.6
	github.com/aws/aws-sdk-go-v2/service/kms v1.45.1
	github.com/aws/smithy-go v1.23.0
	github.com/beevik/ntp v1.4.3
	github.com/benbjohnson/clock v1.3.5 // project archived on 2023-05-18
	github.com/blang/semver/v4 v4.0.0
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/containerd/cgroups/v3 v3.0.5
	github.com/containerd/containerd/api v1.9.0
	github.com/containerd/containerd/v2 v2.1.4
	github.com/containerd/errdefs v1.0.0
	github.com/containerd/log v0.1.0
	github.com/containerd/platforms v1.0.0-rc.1
	github.com/containerd/typeurl/v2 v2.2.3
	github.com/containernetworking/cni v1.3.0
	github.com/containernetworking/plugins v1.8.0
	github.com/coredns/coredns v1.12.3
	github.com/coreos/go-iptables v0.8.0
	github.com/cosi-project/runtime v1.11.0
	github.com/detailyang/go-fallocate v0.0.0-20180908115635-432fa640bd2e
	github.com/distribution/reference v0.6.0
	github.com/docker/cli v28.3.3+incompatible
	github.com/docker/docker v28.3.3+incompatible
	github.com/docker/go-connections v0.6.0
	github.com/dustin/go-humanize v1.0.1
	github.com/elastic/go-libaudit/v2 v2.6.2
	github.com/equinix-ms/go-vmw-guestrpc v0.1.1
	github.com/fatih/color v1.18.0
	github.com/florianl/go-tc v0.4.5
	github.com/foxboron/go-uefi v0.0.0-20250625111927-a3183a1bfc84
	github.com/freddierice/go-losetup/v2 v2.0.1
	github.com/fsnotify/fsnotify v1.9.0
	github.com/g0rbe/go-chattr v1.0.1
	github.com/gdamore/tcell/v2 v2.9.0
	github.com/gertd/go-pluralize v0.2.1
	github.com/gizak/termui/v3 v3.1.0
	github.com/godbus/dbus/v5 v5.1.0
	github.com/golang/mock v1.6.0
	github.com/google/cadvisor v0.53.0
	github.com/google/cel-go v0.26.1
	github.com/google/go-containerregistry v0.20.6
	github.com/google/go-tpm v0.9.5
	github.com/google/nftables v0.3.0
	github.com/google/uuid v1.6.0
	github.com/gopacket/gopacket v1.4.0
	github.com/gosuri/uiprogress v0.0.1
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.2
	github.com/hashicorp/go-cleanhttp v0.5.2
	github.com/hashicorp/go-envparse v0.1.0
	github.com/hashicorp/go-getter/v2 v2.2.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hetznercloud/hcloud-go/v2 v2.22.0
	github.com/insomniacslk/dhcp v0.0.0-20250109001534-8abf58130905
	github.com/jeromer/syslogparser v1.1.0
	github.com/jsimonetti/rtnetlink/v2 v2.0.5
	github.com/jxskiss/base62 v1.1.0
	github.com/klauspost/compress v1.18.0
	github.com/klauspost/cpuid/v2 v2.3.0
	github.com/linode/go-metadata v0.2.2
	github.com/martinlindhe/base36 v1.1.1
	github.com/mattn/go-isatty v0.0.20
	github.com/mdlayher/arp v0.0.0-20220512170110-6706a2966875
	github.com/mdlayher/ethtool v0.4.0
	github.com/mdlayher/genetlink v1.3.2
	github.com/mdlayher/kobject v0.0.0-20200520190114-19ca17470d7d
	github.com/mdlayher/netlink v1.8.0
	github.com/mdlayher/netx v0.0.0-20230430222610-7e21880baee8
	github.com/mdp/qrterminal/v3 v3.2.1
	github.com/miekg/dns v1.1.68
	github.com/nberlee/go-netstat v0.1.2
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/opencontainers/runtime-spec v1.2.1
	github.com/packethost/packngo v0.31.0
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/pin/tftp/v3 v3.1.0
	github.com/pkg/xattr v0.4.12
	github.com/pmorjan/kmod v1.1.1
	github.com/prometheus/procfs v0.17.0
	github.com/rivo/tview v0.42.0
	github.com/rs/xid v1.6.0
	github.com/ryanuber/columnize v2.1.2+incompatible
	github.com/ryanuber/go-glob v1.0.0
	github.com/safchain/ethtool v0.6.2
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.34
	github.com/siderolabs/crypto v0.6.3
	github.com/siderolabs/discovery-api v0.1.6
	github.com/siderolabs/discovery-client v0.1.13
	github.com/siderolabs/gen v0.8.5
	github.com/siderolabs/go-api-signature v0.3.7
	github.com/siderolabs/go-blockdevice v0.4.8
	github.com/siderolabs/go-blockdevice/v2 v2.0.19
	github.com/siderolabs/go-circular v0.2.3
	github.com/siderolabs/go-cmd v0.1.3
	github.com/siderolabs/go-copy v0.1.0
	github.com/siderolabs/go-debug v0.6.0
	github.com/siderolabs/go-kmsg v0.1.4
	github.com/siderolabs/go-kubeconfig v0.1.1
	github.com/siderolabs/go-kubernetes v0.2.26
	github.com/siderolabs/go-loadbalancer v0.5.0
	github.com/siderolabs/go-pcidb v0.3.2
	github.com/siderolabs/go-pointer v1.0.1
	github.com/siderolabs/go-procfs v0.1.2
	github.com/siderolabs/go-retry v0.3.3
	github.com/siderolabs/go-smbios v0.3.3
	github.com/siderolabs/go-tail v0.1.1
	github.com/siderolabs/go-talos-support v0.1.2
	github.com/siderolabs/grpc-proxy v0.5.1
	github.com/siderolabs/kms-client v0.1.0
	github.com/siderolabs/net v0.4.0
	github.com/siderolabs/proto-codec v0.1.2
	github.com/siderolabs/siderolink v0.3.15
	github.com/siderolabs/talos/pkg/machinery v1.12.0-alpha.0
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.10.1
	github.com/spf13/pflag v1.0.9
	github.com/stretchr/testify v1.11.1
	github.com/thejerf/suture/v4 v4.0.6
	github.com/u-root/u-root v0.15.0
	github.com/ulikunitz/xz v0.5.15
	github.com/vultr/metadata v1.1.0
	go.etcd.io/etcd/api/v3 v3.6.4
	go.etcd.io/etcd/client/pkg/v3 v3.6.4
	go.etcd.io/etcd/client/v3 v3.6.4
	go.etcd.io/etcd/etcdutl/v3 v3.6.4
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba
	golang.org/x/net v0.43.0
	golang.org/x/oauth2 v0.30.0
	golang.org/x/sync v0.16.0
	golang.org/x/sys v0.35.0
	golang.org/x/term v0.34.0
	golang.org/x/text v0.28.0
	golang.org/x/time v0.12.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20241231184526-a9ab2273dd10
	google.golang.org/grpc v1.75.0
	google.golang.org/protobuf v1.36.8
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/klog/v2 v2.130.1
	k8s.io/utils v0.0.0-20250820121507-0af2bda4dd1d
	kernel.org/pub/linux/libs/security/libcap/cap v1.2.76
	sigs.k8s.io/hydrophone v0.7.0
)

require (
	cel.dev/expr v0.24.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/internal v1.2.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.4.2 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.13.0 // indirect
	github.com/ProtonMail/go-crypto v1.3.0 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.9.0 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/apparentlymart/go-cidr v1.1.0 // indirect
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2 // indirect
	github.com/aws/aws-sdk-go-v2 v1.38.3 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.18.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.29.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.34.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.38.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/cilium/ebpf v0.19.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/containerd/continuity v0.4.5 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/go-cni v1.1.13 // indirect
	github.com/containerd/plugin v1.0.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.16.3 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.6.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/dot v1.9.0 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-resty/resty/v2 v2.16.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/gosuri/uilive v0.0.4 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hexops/gotextdiff v1.0.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lmittmann/tint v1.0.4 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mdlayher/ethernet v0.0.0-20220221185849-529eae5b6118 // indirect
	github.com/mdlayher/packet v1.1.2 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/nsf/termbox-go v0.0.0-20190121233118-02980233997d // indirect
	github.com/opencontainers/selinux v1.12.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/petermattis/goid v0.0.0-20250508124226-395b08cebbdb // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20241121165744-79df5c4772f2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.23.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.5 // indirect
	github.com/siderolabs/protoenc v0.2.3 // indirect
	github.com/siderolabs/tcpproxy v0.1.0 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/vbatts/tar-split v0.12.1 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xiang90/probing v0.0.0-20221125231312-a49e3df8f510 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	go.etcd.io/bbolt v1.4.3 // indirect
	go.etcd.io/etcd/pkg/v3 v3.6.4 // indirect
	go.etcd.io/etcd/server/v3 v3.6.4 // indirect
	go.etcd.io/raft/v3 v3.6.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/exp v0.0.0-20250506013437-ce4c2cf36ca6 // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20231211153847-12269c276173 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250826171959-ef028d996bc1 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250826171959-ef028d996bc1 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gotest.tools/v3 v3.4.0 // indirect
	k8s.io/cli-runtime v0.34.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250710124328-f3f2b991d03b // indirect
	kernel.org/pub/linux/libs/security/libcap/psx v1.2.76 // indirect
	rsc.io/qr v0.2.0 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/knftables v0.0.18 // indirect
	sigs.k8s.io/kustomize/api v0.20.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.20.1 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

exclude github.com/containerd/containerd v1.7.0
