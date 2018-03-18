package process

import (
	"io/ioutil"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/process/conditions"
)

const MasterConfiguration = `
kind: MasterConfiguration
apiVersion: kubeadm.k8s.io/v1alpha1
criSocket: /var/run/crio/crio.sock
skipTokenPrint: true
kubernetesVersion: v1.10.0-beta.4
networking:
  dnsDomain: cluster.local
  serviceSubnet: 10.96.0.0/12
  podSubnet: 10.244.0.0/16
featureGates:
  HighAvailability: true
  SelfHosting: false
  StoreCertsInSecrets: false
  DynamicKubeletConfig: true
`

type Kubeadm struct{}

func init() {
	if err := ioutil.WriteFile("/etc/kubernetes/kubeadm.yaml", []byte(MasterConfiguration), 0644); err != nil {

	}
}

func (p *Kubeadm) Cmd() (name string, args []string) {
	name = "/bin/kubeadm"
	args = []string{
		"init",
		"--config=/etc/kubernetes/kubeadm.yaml",
	}

	return name, args
}

func (p *Kubeadm) Condition() func() (bool, error) {
	return conditions.WaitForFileExists("/var/run/crio/crio.sock")
}

func (p *Kubeadm) Env() []string { return []string{} }

func (p *Kubeadm) Type() Type { return Once }
