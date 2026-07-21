module github.com/siderolabs/talos

go 1.26.5

replace (
	// forked coredns so we don't carry caddy and other stuff into the Talos
	github.com/coredns/coredns => github.com/siderolabs/coredns v1.14.53

	// forked ethtool introduces missing APIs
	github.com/mdlayher/ethtool => github.com/siderolabs/ethtool v0.6.0-sidero

	// see https://github.com/mdlayher/kobject/pull/5
	github.com/mdlayher/kobject => github.com/smira/kobject v0.0.0-20240304111826-49c8d4613389

	// prevent watcher shutdown leaks until https://github.com/osrg/gobgp/pull/3505 is merged
	github.com/osrg/gobgp/v4 => github.com/frezbo/gobgp/v4 v4.0.0-20260723125704-323c85fa213c

	// replace to disable assembly implementation (see https://github.com/beevik/nts/issues/1#issuecomment-4879122150)
	github.com/secure-io/siv-go => github.com/smira/siv-go v0.0.0-20260706144621-2093d2730928

	// Use nested module.
	github.com/siderolabs/talos/pkg/machinery => ./pkg/machinery

	// fork to add Talos-specific userspace socket location: https://github.com/siderolabs/talos/issues/8514
	golang.zx2c4.com/wireguard/wgctrl => github.com/siderolabs/wgctrl-go v0.0.0-20251029173431-c4fd5f6a4e72
)

// Kubernetes dependencies sharing the same version.
require (
	k8s.io/api v0.37.0-beta.0
	k8s.io/apiextensions-apiserver v0.37.0-beta.0
	k8s.io/apimachinery v0.37.0-beta.0
	k8s.io/apiserver v0.37.0-beta.0
	k8s.io/client-go v0.37.0-beta.0
	k8s.io/component-base v0.37.0-beta.0
	k8s.io/cri-api v0.37.0-beta.0
	k8s.io/kube-proxy v0.37.0-beta.0
	k8s.io/kube-scheduler v0.37.0-beta.0
	k8s.io/kubectl v0.37.0-beta.0
	k8s.io/kubelet v0.37.0-beta.0
	k8s.io/pod-security-admission v0.37.0-beta.0
)

require (
	cloud.google.com/go/compute/metadata v0.9.0
	codeberg.org/miekg/dns v0.6.84
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.22.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.14.0
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates v1.5.0
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys v1.5.0
	github.com/alexflint/go-filemutex v1.3.0
	github.com/aws/aws-sdk-go-v2 v1.43.0
	github.com/aws/aws-sdk-go-v2/config v1.32.31
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.31
	github.com/aws/aws-sdk-go-v2/service/acm v1.43.0
	github.com/aws/aws-sdk-go-v2/service/kms v1.55.0
	github.com/aws/smithy-go v1.27.4
	github.com/beevik/ntp v1.5.0
	github.com/beevik/nts v0.3.1
	github.com/blang/semver/v4 v4.0.0
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/containerd/cgroups/v3 v3.1.3
	github.com/containerd/containerd/api v1.11.1
	github.com/containerd/containerd/v2 v2.3.3
	github.com/containerd/errdefs v1.0.0
	github.com/containerd/log v0.1.0
	github.com/containerd/platforms v1.0.0-rc.4
	github.com/containerd/typeurl/v2 v2.3.0
	github.com/containernetworking/cni v1.3.0
	github.com/containernetworking/plugins v1.9.1
	github.com/coredns/coredns v1.14.6
	github.com/coreos/go-iptables v0.8.0
	github.com/cosi-project/runtime v1.16.2
	github.com/detailyang/go-fallocate v0.0.0-20180908115635-432fa640bd2e
	github.com/distribution/reference v0.6.0
	github.com/docker/cli v29.6.2+incompatible
	github.com/dustin/go-humanize v1.0.1
	github.com/elastic/go-libaudit/v2 v2.6.2
	github.com/equinix-ms/go-vmw-guestrpc v1.0.0
	github.com/fatih/color v1.19.0
	github.com/florianl/go-tc v0.4.8
	github.com/foxboron/go-uefi v0.0.0-20251010190908-d29549a44f29
	github.com/freddierice/go-losetup/v2 v2.0.1
	github.com/fsnotify/fsnotify v1.10.1
	github.com/g0rbe/go-chattr v1.0.1
	github.com/gdamore/tcell/v2 v2.13.10
	github.com/gertd/go-pluralize v0.2.1
	github.com/godbus/dbus/v5 v5.2.2
	github.com/golang/mock v1.7.0-rc.1
	github.com/google/cadvisor/lib v0.60.5
	github.com/google/cel-go v0.29.2
	github.com/google/go-containerregistry v0.21.7
	github.com/google/go-tpm v0.9.8
	github.com/google/nftables v0.3.0
	github.com/google/uuid v1.6.0
	github.com/gopacket/gopacket v1.7.0
	github.com/gosuri/uiprogress v0.0.1
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.3
	github.com/hashicorp/go-cleanhttp v0.5.2
	github.com/hashicorp/go-envparse v0.1.0
	github.com/hashicorp/go-getter/v2 v2.2.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hetznercloud/hcloud-go/v2 v2.45.0
	github.com/insomniacslk/dhcp v0.0.0-20260719225207-c76316d4aa82
	github.com/jeromer/syslogparser v1.1.0
	github.com/jsimonetti/rtnetlink/v2 v2.2.1-0.20260714114318-c87a4183a51a
	github.com/jxskiss/base62 v1.1.0
	github.com/klauspost/compress v1.19.1
	github.com/klauspost/cpuid/v2 v2.4.0
	github.com/linode/go-metadata v0.3.0
	github.com/martinlindhe/base36 v1.1.1
	github.com/mattn/go-isatty v0.0.23
	github.com/mdlayher/arp v0.0.0-20260528070854-93566ba168e9
	github.com/mdlayher/ethtool v0.6.1
	github.com/mdlayher/genetlink v1.4.0
	github.com/mdlayher/kobject v0.0.0-20200520190114-19ca17470d7d
	github.com/mdlayher/ndp v1.1.0
	github.com/mdlayher/netlink v1.11.2
	github.com/mdlayher/netx v0.0.0-20230430222610-7e21880baee8
	github.com/mdp/qrterminal/v3 v3.2.1
	github.com/miekg/dns v1.1.72
	github.com/moby/moby/api v1.55.0
	github.com/moby/moby/client v0.5.0
	github.com/navidys/tvxwidgets v0.14.0
	github.com/nberlee/go-netstat v0.1.2
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/osrg/gobgp/v4 v4.7.0
	github.com/packethost/packngo v0.31.0
	github.com/pelletier/go-toml/v2 v2.4.3
	github.com/pin/tftp/v3 v3.2.0
	github.com/pkg/xattr v0.4.12
	github.com/planetscale/vtprotobuf v0.6.1-0.20260702190614-8ae5a48058df
	github.com/pmorjan/kmod v1.1.1
	github.com/prometheus/procfs v0.21.1
	github.com/rivo/tview v0.42.0
	github.com/rs/xid v1.6.0
	github.com/ryanuber/go-glob v1.0.0
	github.com/safchain/ethtool v0.7.0
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.36
	github.com/siderolabs/crypto v0.6.5
	github.com/siderolabs/discovery-api v0.1.8
	github.com/siderolabs/discovery-client v0.1.15
	github.com/siderolabs/gen v0.8.7
	github.com/siderolabs/go-adv v1.0.0
	github.com/siderolabs/go-blockdevice/v2 v2.0.32
	github.com/siderolabs/go-circular v0.2.3
	github.com/siderolabs/go-cmd v0.2.1
	github.com/siderolabs/go-copy v0.1.0
	github.com/siderolabs/go-debug v0.6.2
	github.com/siderolabs/go-kmsg v0.1.6
	github.com/siderolabs/go-kubeconfig v0.1.2
	github.com/siderolabs/go-kubernetes v0.2.41
	github.com/siderolabs/go-loadbalancer v0.5.0
	github.com/siderolabs/go-pcidb v0.3.3
	github.com/siderolabs/go-pointer v1.0.1
	github.com/siderolabs/go-procfs v0.1.2
	github.com/siderolabs/go-retry v0.3.3
	github.com/siderolabs/go-smbios v0.3.4
	github.com/siderolabs/go-tail v0.1.1
	github.com/siderolabs/go-talos-support v0.3.0
	github.com/siderolabs/grpc-proxy v0.5.2
	github.com/siderolabs/kms-client v0.2.0
	github.com/siderolabs/net v0.4.0
	github.com/siderolabs/proto-codec v0.1.4
	github.com/siderolabs/siderolink v0.3.16
	github.com/siderolabs/talos/pkg/machinery v1.14.0-beta.0
	github.com/sigstore/cosign/v3 v3.1.2
	github.com/sigstore/sigstore v1.10.8
	github.com/sigstore/sigstore-go v1.2.2
	github.com/sirupsen/logrus v1.9.4
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.11.1
	github.com/thejerf/suture/v4 v4.0.6
	github.com/theupdateframework/go-tuf/v2 v2.4.2
	github.com/u-root/u-root v0.16.0
	github.com/ulikunitz/xz v0.5.16
	github.com/vultr/metadata v1.1.0
	go.etcd.io/etcd/api/v3 v3.7.0
	go.etcd.io/etcd/client/pkg/v3 v3.7.0
	go.etcd.io/etcd/client/v3 v3.7.0
	go.etcd.io/etcd/etcdutl/v3 v3.7.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
	go.uber.org/zap/exp v0.3.0
	go.yaml.in/yaml/v4 v4.0.0-rc.6
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba
	golang.org/x/net v0.57.0
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sync v0.22.0
	golang.org/x/sys v0.47.0
	golang.org/x/term v0.45.0
	golang.org/x/text v0.40.0
	golang.org/x/time v0.15.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20241231184526-a9ab2273dd10
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af
	gopkg.in/typ.v4 v4.4.0
	k8s.io/klog/v2 v2.140.0
	kernel.org/pub/linux/libs/security/libcap/cap v1.2.78
	sigs.k8s.io/hydrophone v0.7.0
	sigs.k8s.io/yaml v1.6.0
)

require (
	cel.dev/expr v0.25.2 // indirect
	filippo.io/age v1.3.1 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	filippo.io/hpke v0.4.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.12.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/internal v1.2.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.7.2 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.3-0.20251027160822-ad3df93bed29 // indirect
	github.com/Microsoft/hcsshim v0.15.0-rc.3 // indirect
	github.com/ProtonMail/go-crypto v1.4.1 // indirect
	github.com/ProtonMail/gopenpgp/v3 v3.4.1 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/aead/cmac v0.0.0-20160719120800-7af84192f0b1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/apparentlymart/go-cidr v1.1.1 // indirect
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.31 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.31 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.32 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.5.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.33.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.38.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.45.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chai2010/gettext-go v1.0.3 // indirect
	github.com/cilium/ebpf v0.22.0 // indirect
	github.com/cloudflare/circl v1.6.4 // indirect
	github.com/containerd/continuity v0.5.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/go-cni v1.1.13 // indirect
	github.com/containerd/plugin v1.1.0 // indirect
	github.com/containerd/ttrpc v1.2.9 // indirect
	github.com/coreos/go-oidc/v3 v3.20.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-farm v0.0.0-20240924180020-3414d57e47da // indirect
	github.com/digitorus/pkcs7 v0.0.0-20250730155240-ffadbf3f398c // indirect
	github.com/digitorus/timestamp v0.0.0-20250524132541-c45532741eea // indirect
	github.com/docker/docker-credential-helpers v0.9.8 // indirect
	github.com/docker/go-connections v0.7.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/eapache/channels v1.1.0 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/emicklei/dot v1.11.0 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fluxcd/cli-utils v1.2.2 // indirect
	github.com/fluxcd/pkg/ssa v0.77.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.2 // indirect
	github.com/gaissmai/bart v0.26.1 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-chi/chi/v5 v5.3.1 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.25.3 // indirect
	github.com/go-openapi/errors v0.22.8 // indirect
	github.com/go-openapi/jsonpointer v1.0.0 // indirect
	github.com/go-openapi/jsonreference v1.0.0 // indirect
	github.com/go-openapi/loads v0.24.0 // indirect
	github.com/go-openapi/runtime v0.32.4 // indirect
	github.com/go-openapi/runtime/server-middleware v0.32.4 // indirect
	github.com/go-openapi/spec v0.22.6 // indirect
	github.com/go-openapi/strfmt v0.26.4 // indirect
	github.com/go-openapi/swag v0.27.0 // indirect
	github.com/go-openapi/swag/cmdutils v0.27.0 // indirect
	github.com/go-openapi/swag/conv v0.27.0 // indirect
	github.com/go-openapi/swag/fileutils v0.27.0 // indirect
	github.com/go-openapi/swag/jsonname v0.27.0 // indirect
	github.com/go-openapi/swag/jsonutils v0.27.0 // indirect
	github.com/go-openapi/swag/loading v0.27.0 // indirect
	github.com/go-openapi/swag/mangling v0.27.0 // indirect
	github.com/go-openapi/swag/netutils v0.27.0 // indirect
	github.com/go-openapi/swag/stringutils v0.27.0 // indirect
	github.com/go-openapi/swag/typeutils v0.27.0 // indirect
	github.com/go-openapi/swag/yamlutils v0.27.0 // indirect
	github.com/go-openapi/validate v0.26.0 // indirect
	github.com/go-resty/resty/v2 v2.17.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/certificate-transparency-go v1.3.3 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/gosuri/uilive v0.0.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/in-toto/attestation v1.2.0 // indirect
	github.com/in-toto/in-toto-golang v0.11.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jedisct1/go-minisign v0.0.0-20260527172527-a09352b57a22 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/k-sone/critbitgo v1.4.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/letsencrypt/boulder v0.20260713.0 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lmittmann/tint v1.2.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mdlayher/ethernet v0.0.0-20220221185849-529eae5b6118 // indirect
	github.com/mdlayher/packet v1.1.2 // indirect
	github.com/mdlayher/socket v0.6.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/spdystream v0.5.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/sequential v0.7.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/moby/sys/user v0.4.1 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/neticdk/go-stdlib v1.0.1 // indirect
	github.com/nozzle/throttler v0.0.0-20180817012639-2ea982251481 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/petermattis/goid v0.0.0-20260713124913-97594f28f5ca // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.9 // indirect
	github.com/sassoftware/relic v7.2.1+incompatible // indirect
	github.com/secure-io/siv-go v0.0.0-20180922214919-5ff40651e2c4 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.11.0 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/siderolabs/go-api-signature v0.3.13 // indirect
	github.com/siderolabs/protoenc v0.2.4 // indirect
	github.com/siderolabs/tcpproxy v0.1.0 // indirect
	github.com/sigstore/protobuf-specs v0.5.1 // indirect
	github.com/sigstore/rekor v1.5.3 // indirect
	github.com/sigstore/rekor-tiles/v2 v2.3.0 // indirect
	github.com/sigstore/timestamp-authority/v2 v2.1.3 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/theupdateframework/go-tuf v0.7.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/transparency-dev/formats v0.1.1 // indirect
	github.com/transparency-dev/merkle v0.0.2 // indirect
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/wI2L/jsondiff v0.6.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xiang90/probing v0.0.0-20221125231312-a49e3df8f510 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	github.com/zalando/go-keyring v0.2.8 // indirect
	go.etcd.io/bbolt v1.5.0 // indirect
	go.etcd.io/etcd/pkg/v3 v3.7.0 // indirect
	go.etcd.io/etcd/server/v3 v3.7.0 // indirect
	go.etcd.io/raft/v3 v3.7.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.69.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.69.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/exp v0.0.0-20260709172345-9ea1abe57597 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20260522210424-ecfc5a8d5446 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260720211330-0afa2a65878a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260720211330-0afa2a65878a // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/cli-runtime v0.37.0-beta.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260706235625-cdb1db5517a0 // indirect
	k8s.io/streaming v0.37.0-beta.0 // indirect
	k8s.io/utils v0.0.0-20260707023825-cf1189d6abe3 // indirect
	kernel.org/pub/linux/libs/security/libcap/psx v1.2.78 // indirect
	rsc.io/qr v0.2.0 // indirect
	sigs.k8s.io/controller-runtime v0.24.1 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/knftables v0.0.21 // indirect
	sigs.k8s.io/kustomize/api v0.21.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.21.1 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
)

exclude github.com/containerd/containerd v1.7.0
