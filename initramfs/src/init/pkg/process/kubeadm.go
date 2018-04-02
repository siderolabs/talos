package process

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"text/template"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/process/conditions"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
)

const MasterConfiguration = `
kind: MasterConfiguration
apiVersion: kubeadm.k8s.io/v1alpha1
token: {{ .Token }}
TokenTTL: 0s
criSocket: /var/run/crio/crio.sock
skipTokenPrint: true
kubernetesVersion: v1.10.0
networking:
  dnsDomain: cluster.local
  serviceSubnet: 10.96.0.0/12
  podSubnet: 10.244.0.0/16
featureGates:
  HighAvailability: true
  SelfHosting: false
  StoreCertsInSecrets: false
  DynamicKubeletConfig: true
  CoreDNS: true
`

const NodeConfiguration = `
kind: NodeConfiguration
apiVersion: kubeadm.k8s.io/v1alpha1
token: {{ .Token }}
criSocket: /var/run/crio/crio.sock
discoveryTokenAPIServers:
  - {{ .APIServer }}
discoveryTokenUnsafeSkipCAVerification: true
nodeName: {{ .NodeName }}
`

type Kubeadm struct{}

var cmd string

func (p *Kubeadm) Pre(data userdata.UserData) error {
	var configuration string
	if data.Join {
		configuration = NodeConfiguration
	} else {
		configuration = MasterConfiguration
	}

	tmpl, err := template.New("").Parse(configuration)
	if err != nil {
		return err
	}
	var buf []byte
	writer := bytes.NewBuffer(buf)
	err = tmpl.Execute(writer, data)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile("/etc/kubernetes/kubeadm.yaml", writer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write kubeadm.yaml: %s", err.Error())
	}

	// TODO: "modprobe -a ip_vs ip_vs_rr ip_vs_wrr ip_vs_sh nf_conntrack_ipv4"

	return nil
}

func (p *Kubeadm) Cmd(data userdata.UserData) (name string, args []string) {
	var cmd string
	if data.Join {
		cmd = "join"
	} else {
		cmd = "init"
	}
	name = "/bin/kubeadm"
	args = []string{
		cmd,
		"--config=/etc/kubernetes/kubeadm.yaml",
		"--ignore-preflight-errors=cri",
	}

	return name, args
}

func (p *Kubeadm) Condition() func() (bool, error) {
	return conditions.WaitForFileExists("/var/run/crio/crio.sock")
}

func (p *Kubeadm) Env() []string { return []string{} }

func (p *Kubeadm) Type() Type { return Once }
