module github.com/talos-systems/talos

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/Microsoft/go-winio v0.4.9 // indirect
	github.com/Microsoft/hcsshim v0.7.0 // indirect
	github.com/beevik/ntp v0.2.0
	github.com/containerd/cgroups v0.0.0-20180905221500-58556f5ad844
	github.com/containerd/containerd v1.2.5
	github.com/containerd/continuity v0.0.0-20181003075958-be9bd761db19 // indirect
	github.com/containerd/cri v1.11.1
	github.com/containerd/fifo v0.0.0-20180307165137-3d5202aec260 // indirect
	github.com/containerd/typeurl v0.0.0-20180627222232-a93fcdb778cd
	github.com/coreos/go-systemd v0.0.0-20180828140353-eee3db372b31 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible // indirect
	github.com/docker/go-events v0.0.0-20170721190031-9461782956ad // indirect
	github.com/docker/go-units v0.3.3 // indirect
	github.com/evanphx/json-patch v4.1.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.4.7 // indirect
	github.com/fullsailor/pkcs7 v0.0.0-20180613152042-8306686428a5
	github.com/godbus/dbus v4.1.0+incompatible // indirect
	github.com/gogo/googleapis v1.1.0 // indirect
	github.com/gogo/protobuf v1.1.1 // indirect
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/golang/protobuf v1.2.0
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf // indirect
	github.com/google/uuid v1.0.0
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/hashicorp/golang-lru v0.5.0 // indirect
	github.com/hpcloud/tail v1.0.0 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/json-iterator/go v0.0.0-20180701071628-ab8a2e0c74be // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/lithammer/dedent v1.1.0 // indirect
	github.com/mdlayher/genetlink v0.0.0-20190313224034-60417448a851
	github.com/mdlayher/netlink v0.0.0-20190313131330-258ea9dff42c
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742 // indirect
	github.com/onsi/ginkgo v1.6.0 // indirect
	github.com/onsi/gomega v1.4.1 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/opencontainers/runtime-spec v0.1.2-0.20180710222632-d810dbc60d8c
	github.com/pkg/errors v0.8.1
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.0.6 // indirect
	github.com/spf13/afero v1.2.0 // indirect
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/stretchr/objx v0.1.1 // indirect
	github.com/stretchr/testify v1.2.2
	github.com/syndtr/gocapability v0.0.0-20180223013746-33e07d32887e // indirect
	github.com/talos-systems/dhcp v0.0.0-20190403231749-dd8bdda8e381
	github.com/u-root/u-root v4.0.0+incompatible // indirect
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20180720170159-13995c7128cc // indirect
	github.com/vmware/vmw-guestinfo v0.0.0-20170707015358-25eff159a728
	golang.org/x/sync v0.0.0-20181108010431-42b317875d0f // indirect
	golang.org/x/sys v0.0.0-20190312061237-fead79001313
	golang.org/x/text v0.3.0
	golang.org/x/time v0.0.0-20181108054448-85acf8d2951c // indirect
	google.golang.org/genproto v0.0.0-20181221175505-bd9b4fb69e2f // indirect
	google.golang.org/grpc v1.17.0
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gotest.tools v2.1.0+incompatible // indirect
	k8s.io/api v0.0.0-20190313235455-40a48860b5ab
	k8s.io/apiextensions-apiserver v0.0.0-20190322231200-1c09d17c1352 // indirect
	k8s.io/apimachinery v0.0.0-20190313205120-d7deff9243b1
	k8s.io/apiserver v0.0.0-20190324105220-f881eae9ec04 // indirect
	k8s.io/client-go v2.0.0-alpha.0.0.20190313235726-6ee68ca5fd83+incompatible
	k8s.io/cloud-provider v0.0.0-20190323031113-9c9d72d1bf90 // indirect
	k8s.io/cluster-bootstrap v0.0.0-20190313124217-0fa624df11e9 // indirect
	k8s.io/component-base v0.0.0-20190313120452-4727f38490bc // indirect
	k8s.io/klog v0.2.0 // indirect
	k8s.io/kube-openapi v0.0.0-20190320154901-5e45bb682580 // indirect
	k8s.io/kube-proxy v0.0.0-20190320190624-78a1c9778e0e // indirect
	k8s.io/kubelet v0.0.0-20190313123811-3556bcde9670 // indirect
	k8s.io/kubernetes v1.14.0
	k8s.io/utils v0.0.0-20190308190857-21c4ce38f2a7 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)
