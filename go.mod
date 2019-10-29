module github.com/talos-systems/talos

go 1.13

replace github.com/jsimonetti/rtnetlink => github.com/bradbeam/rtnetlink v0.0.0-20190820045831-7b9ca088b93d

replace github.com/kubernetes-incubator/bootkube => github.com/andrewrynhard/bootkube v0.14.1-0.20191015145817-ac01e28e2840

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20180906201452-2aa6f33b730c
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/Microsoft/hcsshim v0.8.6 // indirect
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/beevik/ntp v0.2.0
	github.com/containerd/cgroups v0.0.0-20190328223300-4994991857f9
	github.com/containerd/containerd v1.2.7
	github.com/containerd/continuity v0.0.0-20181003075958-be9bd761db19 // indirect
	github.com/containerd/cri v1.11.1
	github.com/containerd/fifo v0.0.0-20180307165137-3d5202aec260 // indirect
	github.com/containerd/typeurl v0.0.0-20190228175220-2a93cfde8c20
	github.com/coreos/etcd v3.3.15+incompatible // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-events v0.0.0-20170721190031-9461782956ad // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fullsailor/pkcs7 v0.0.0-20180613152042-8306686428a5
	github.com/gizak/termui/v3 v3.0.0
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/gogo/googleapis v1.1.0 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/btree v1.0.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hugelgupf/socketpair v0.0.0-20190730060125-05d35a94e714 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20190814082028-393ae75a101b
	github.com/jsimonetti/rtnetlink v0.0.0-20190606172950-9527aa82566a
	github.com/kubernetes-incubator/bootkube v0.14.1-0.20190731222813-f0fc1bdb404d
	github.com/mdlayher/ethernet v0.0.0-20190606142754-0394541c37b7 // indirect
	github.com/mdlayher/genetlink v0.0.0-20190313224034-60417448a851
	github.com/mdlayher/netlink v0.0.0-20190419142405-71c9566a34ae
	github.com/mdlayher/raw v0.0.0-20190606144222-a54781e5f38f // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v1.0.0-rc8 // indirect
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/prometheus/procfs v0.0.3
	github.com/ryanuber/columnize v2.1.0+incompatible
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/gocapability v0.0.0-20180223013746-33e07d32887e
	github.com/u-root/u-root v6.0.0+incompatible // indirect
	github.com/vmware/vmw-guestinfo v0.0.0-20170707015358-25eff159a728
	go.etcd.io/etcd v3.3.13+incompatible
	golang.org/x/crypto v0.0.0-20190820162420-60c769a6c586
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7 // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190825160603-fb81701db80f
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/grpc v1.23.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/freddierice/go-losetup.v1 v1.0.0-20170407175016-fc9adea44124
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0-20190918155943-95b840bb6a1f
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
	k8s.io/cri-api v0.0.0-20190828162817-608eb1dad4ac
)

replace (
	github.com/docker/distribution v2.7.1+incompatible => github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
	github.com/opencontainers/runtime-spec v1.0.1 => github.com/opencontainers/runtime-spec v0.1.2-0.20180301181910-fa4b36aa9c99
)
