module github.com/talos-systems/talos

go 1.17

replace (
	// Use nested module.
	github.com/talos-systems/talos/pkg/machinery => ./pkg/machinery

	// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
	// which are then encoded as a valid YAML blocks with proper indentiation
	gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20210315173758-8fb30b8e5a5b

	// See https://github.com/talos-systems/go-loadbalancer/pull/4
	// `go get github.com/smira/tcpproxy@combined-fixes`, then copy pseudo-version there
	inet.af/tcpproxy => github.com/smira/tcpproxy v0.0.0-20201015133617-de5f7797b95b
)

// Kubernetes dependencies sharing the same version.
require (
	k8s.io/api v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/apiserver v0.23.1 // indirect
	k8s.io/cli-runtime v0.23.1 // indirect
	k8s.io/client-go v0.23.1
	k8s.io/component-base v0.23.1
	k8s.io/cri-api v0.23.1
	k8s.io/kubectl v0.23.1
	k8s.io/kubelet v0.23.1
)

require (
	github.com/AlekSi/pointer v1.2.0
	github.com/BurntSushi/toml v0.4.1
	github.com/beevik/ntp v0.3.0
	github.com/cenkalti/backoff/v4 v4.1.2
	github.com/containerd/cgroups v1.0.2
	github.com/containerd/containerd v1.5.8
	github.com/containerd/cri v1.19.0
	github.com/containerd/typeurl v1.0.2
	github.com/containernetworking/cni v1.0.1
	github.com/containernetworking/plugins v1.0.1
	github.com/coreos/go-iptables v0.6.0
	github.com/coreos/go-semver v0.3.0
	github.com/cosi-project/runtime v0.0.0-20211216175730-264f8fcd1a4f
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.11+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/dustin/go-humanize v1.0.0
	github.com/emicklei/dot v0.16.0
	github.com/evanphx/json-patch v5.6.0+incompatible
	github.com/fatih/color v1.13.0
	github.com/fsnotify/fsnotify v1.5.1
	github.com/fullsailor/pkcs7 v0.0.0-20190404230743-d7302db945fa
	github.com/gdamore/tcell/v2 v2.4.1-0.20210905002822-f057f0a857a1
	github.com/gizak/termui/v3 v3.1.0
	github.com/google/go-cmp v0.5.6
	github.com/google/nftables v0.0.0-20210916140115-16a134723a96
	github.com/google/uuid v1.3.0
	github.com/gosuri/uiprogress v0.0.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/hashicorp/go-getter v1.5.9
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hetznercloud/hcloud-go v1.33.1
	github.com/insomniacslk/dhcp v0.0.0-20211214070828-5297eed8f489
	github.com/jsimonetti/rtnetlink v0.0.0-20211203074127-fd9a11f42291
	github.com/jxskiss/base62 v1.0.0
	github.com/mattn/go-isatty v0.0.14
	github.com/mdlayher/arp v0.0.0-20191213142603-f72070a231fc
	github.com/mdlayher/ethtool v0.0.0-20211028163843-288d040e9d60
	github.com/mdlayher/genetlink v1.0.0
	github.com/mdlayher/netlink v1.4.2
	github.com/mdlayher/netx v0.0.0-20200512211805-669a06fde734
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/packethost/packngo v0.20.0
	github.com/pin/tftp v2.1.0+incompatible
	github.com/prometheus/procfs v0.7.3
	github.com/rivo/tview v0.0.0-20211202162923-2a6de950f73b
	github.com/rs/xid v1.3.0
	github.com/ryanuber/columnize v2.1.2+incompatible
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.7
	github.com/spf13/cobra v1.2.1
	github.com/stretchr/testify v1.7.0
	github.com/talos-systems/crypto v0.3.4
	github.com/talos-systems/discovery-api v0.1.0
	github.com/talos-systems/discovery-client v0.1.0
	github.com/talos-systems/go-blockdevice v0.2.6-0.20211214184027-6928ee43c303
	github.com/talos-systems/go-cmd v0.1.0
	github.com/talos-systems/go-debug v0.2.1
	github.com/talos-systems/go-kmsg v0.1.1
	github.com/talos-systems/go-loadbalancer v0.1.1
	github.com/talos-systems/go-procfs v0.1.0
	github.com/talos-systems/go-retry v0.3.1
	github.com/talos-systems/go-smbios v0.1.1
	github.com/talos-systems/grpc-proxy v0.2.0
	github.com/talos-systems/net v0.3.1
	github.com/talos-systems/siderolink v0.1.0
	github.com/talos-systems/talos/pkg/machinery v0.14.0-alpha.2
	github.com/u-root/u-root v7.0.0+incompatible
	github.com/vishvananda/netlink v1.1.1-0.20210330154013-f5de75959ad5
	github.com/vmware-tanzu/sonobuoy v0.55.1
	github.com/vmware/govmomi v0.27.2
	github.com/vmware/vmw-guestinfo v0.0.0-20211006225857-cc1fd90d572c
	github.com/vultr/metadata v1.0.3
	go.etcd.io/etcd/api/v3 v3.5.1
	go.etcd.io/etcd/client/pkg/v3 v3.5.1
	go.etcd.io/etcd/client/v3 v3.5.1
	go.etcd.io/etcd/etcdutl/v3 v3.5.1
	go.uber.org/atomic v1.9.0
	go.uber.org/zap v1.19.1
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211205182925-97ca703d548d
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20211124212657-dd7407c86d22
	google.golang.org/grpc v1.42.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/freddierice/go-losetup.v1 v1.0.0-20170407175016-fc9adea44124
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	inet.af/netaddr v0.0.0-20211027220019-c74959edd3b6
	kernel.org/pub/linux/libs/security/libcap/cap v1.2.61
	sigs.k8s.io/yaml v1.3.0
)

require (
	cloud.google.com/go v0.81.0 // indirect
	cloud.google.com/go/storage v1.10.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.13 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd // indirect
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/Microsoft/hcsshim v0.8.23 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2 // indirect
	github.com/aws/aws-sdk-go v1.15.78 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/bits-and-blooms/bitset v1.2.0 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/briandowns/spinner v1.6.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/chai2010/gettext-go v0.0.0-20160711120539-c6fed771bfd5 // indirect
	github.com/cilium/ebpf v0.7.0 // indirect
	github.com/containerd/continuity v0.1.0 // indirect
	github.com/containerd/fifo v1.0.0 // indirect
	github.com/containerd/go-cni v1.1.0 // indirect
	github.com/containerd/ttrpc v1.1.0 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/digitalocean/go-smbios v0.0.0-20180907143718-390a4f403a8e // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/emicklei/go-restful v2.9.5+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/form3tech-oss/jwt-go v3.2.3+incompatible // indirect
	github.com/fvbommel/sortorder v1.0.1 // indirect
	github.com/gdamore/encoding v1.0.0 // indirect
	github.com/gertd/go-pluralize v0.1.7 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-errors/errors v1.0.1 // indirect
	github.com/go-logr/logr v1.2.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.14 // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/gax-go/v2 v2.0.5 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gosuri/uilive v0.0.4 // indirect
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.0 // indirect
	github.com/hashicorp/go-memdb v1.3.2 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.0.0-20160803190731-bd40a432e4c7 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/josharian/native v0.0.0-20200817173448-b6b71def0850 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/jstemmer/go-junit-report v0.9.1 // indirect
	github.com/klauspost/compress v1.11.13 // indirect
	github.com/koneu/natend v0.0.0-20150829182554-ec0926ea948d // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/mattn/go-colorable v0.1.9 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mdlayher/ethernet v0.0.0-20190606142754-0394541c37b7 // indirect
	github.com/mdlayher/raw v0.0.0-20191009151244-50f2db8cc065 // indirect
	github.com/mdlayher/socket v0.0.0-20211102153432-57e3fa563ecb // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.0.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.4.1 // indirect
	github.com/moby/term v0.0.0-20210610120745-9d4ed1856297 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/nsf/termbox-go v0.0.0-20190121233118-02980233997d // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v1.0.2 // indirect
	github.com/opencontainers/selinux v1.8.2 // indirect
	github.com/pelletier/go-toml v1.9.3 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.11.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.28.0 // indirect
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday v1.5.2 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/satori/go.uuid v1.2.1-0.20181028125025-b2ce2384e17b // indirect
	github.com/sethgrid/pester v0.0.0-20190127155807-68a33a018ad0 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.8.1 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/u-root/uio v0.0.0-20210528114334-82958018845c // indirect
	github.com/ulikunitz/xz v0.5.8 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	github.com/xlab/treeprint v0.0.0-20181112141820-a009c3971eca // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.etcd.io/etcd/client/v2 v2.305.1 // indirect
	go.etcd.io/etcd/pkg/v3 v3.5.1 // indirect
	go.etcd.io/etcd/raft/v3 v3.5.1 // indirect
	go.etcd.io/etcd/server/v3 v3.5.1 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/contrib v0.20.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.20.0 // indirect
	go.opentelemetry.io/otel v0.20.0 // indirect
	go.opentelemetry.io/otel/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/trace v0.20.0 // indirect
	go.starlark.net v0.0.0-20200306205701-8dd3e2ee1dd5 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go4.org/intern v0.0.0-20211027215823-ae77deb06f29 // indirect
	go4.org/unsafe/assume-no-moving-gc v0.0.0-20211027215541-db492cf91b37 // indirect
	golang.org/x/crypto v0.0.0-20211117183948-ae814b36b871 // indirect
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/mod v0.5.1 // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/tools v0.1.7 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20211123210315-387f7c461a16 // indirect
	google.golang.org/api v0.44.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20211203200212-54befc351ae9 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	honnef.co/go/tools v0.2.2 // indirect
	inet.af/tcpproxy v0.0.0-20200125044825-b6bb9b5b8252 // indirect
	k8s.io/klog v1.0.0 // indirect
	k8s.io/klog/v2 v2.30.0 // indirect
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65 // indirect
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b // indirect
	kernel.org/pub/linux/libs/security/libcap/psx v1.2.61 // indirect
	sigs.k8s.io/json v0.0.0-20211020170558-c049b76a60c6 // indirect
	sigs.k8s.io/kustomize/api v0.10.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.13.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
)
