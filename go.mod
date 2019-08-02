module github.com/talos-systems/talos

go 1.12

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20180906201452-2aa6f33b730c
	github.com/beevik/ntp v0.2.0
	github.com/containerd/cgroups v0.0.0-20190328223300-4994991857f9
	github.com/containerd/containerd v1.2.7
	github.com/containerd/continuity v0.0.0-20181003075958-be9bd761db19 // indirect
	github.com/containerd/cri v1.11.1
	github.com/containerd/fifo v0.0.0-20180307165137-3d5202aec260 // indirect
	github.com/containerd/typeurl v0.0.0-20190228175220-2a93cfde8c20
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/go-systemd v0.0.0-20180828140353-eee3db372b31 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-events v0.0.0-20170721190031-9461782956ad // indirect
	github.com/fullsailor/pkcs7 v0.0.0-20180613152042-8306686428a5
	github.com/gizak/termui/v3 v3.0.0
	github.com/gogo/googleapis v1.1.0 // indirect
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/golang/protobuf v1.3.1
	github.com/google/btree v1.0.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.9.2 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/hugelgupf/socketpair v0.0.0-20190730060125-05d35a94e714 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20190814082028-393ae75a101b
	github.com/jsimonetti/rtnetlink v0.0.0-20190606172950-9527aa82566a
	github.com/mdlayher/ethernet v0.0.0-20190606142754-0394541c37b7 // indirect
	github.com/mdlayher/genetlink v0.0.0-20190313224034-60417448a851
	github.com/mdlayher/netlink v0.0.0-20190419142405-71c9566a34ae
	github.com/mdlayher/raw v0.0.0-20190606144222-a54781e5f38f // indirect
	github.com/opencontainers/runc v1.0.0-rc8 // indirect
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.0.0 // indirect
	github.com/prometheus/procfs v0.0.2
	github.com/ryanuber/columnize v2.1.0+incompatible
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spf13/cobra v0.0.4
	github.com/stretchr/testify v1.3.1-0.20190311161405-34c6fa2dc709
	github.com/syndtr/gocapability v0.0.0-20180223013746-33e07d32887e // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/u-root/u-root v6.0.0+incompatible // indirect
	github.com/vmware/vmw-guestinfo v0.0.0-20170707015358-25eff159a728
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	go.etcd.io/etcd v3.3.13+incompatible
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20190611184440-5c40567a22f8
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859 // indirect
	golang.org/x/sys v0.0.0-20190616124812-15dcb6c0061f
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	golang.org/x/xerrors v0.0.0-20190410155217-1f06c39b4373
	google.golang.org/genproto v0.0.0-20190508193815-b515fa19cec8 // indirect
	google.golang.org/grpc v1.20.1
	gopkg.in/freddierice/go-losetup.v1 v1.0.0-20170407175016-fc9adea44124
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/cri-api v0.0.0
	k8s.io/kubernetes v1.16.0-alpha.3
	k8s.io/utils v0.0.0 // indirect
)

replace (
	github.com/docker/distribution v2.7.1+incompatible => github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
	github.com/opencontainers/runtime-spec v1.0.1 => github.com/opencontainers/runtime-spec v0.1.2-0.20180301181910-fa4b36aa9c99
	k8s.io/api => k8s.io/api v0.0.0-20190807220816-c6187fd08fae
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190807222025-4ce8d22863b9
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190806215851-162a2dabc72f
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190807221403-0d1bd8f29a96
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190814105353-799c3f7793f3
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190807222902-8ab00327a9c5
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190807222806-e2c2aeea42c0
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190803082810-c4ef572adb98
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190814102701-2c85c628702c
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190802024754-ba79db1301b4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190807222956-a62f17e61b50
	k8s.io/klog => k8s.io/klog v0.3.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190807221533-fdac087d9dd2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190807222716-fe4c9e4295bd
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190807222432-61b023e833c5
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190807222621-642e553af6d6
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20190807223350-296b7feb3712
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190807222527-c75499879fd7
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190807223058-bf918bc72210
	k8s.io/metrics => k8s.io/metrics v0.0.0-20190814142237-48afa1f27dbd
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190807221700-8ca4988dc682
	k8s.io/utils => k8s.io/utils v0.0.0-20190801114015-581e00157fb1
)
