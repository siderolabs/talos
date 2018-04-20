package process

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"text/template"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/process/conditions"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
)

const MasterConfiguration = `
kind: MasterConfiguration
apiVersion: kubeadm.k8s.io/v1alpha1
token: {{ .Token }}
TokenTTL: 0s
criSocket: {{ .CRISocket }}
skipTokenPrint: true
kubernetesVersion: v1.10.1
networking:
  dnsDomain: cluster.local
  serviceSubnet: 10.96.0.0/12
  podSubnet: 10.244.0.0/16
kubeProxy:
  config:
    mode: ipvs
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
criSocket: {{ .CRISocket }}
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

	var criSocket string
	switch data.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		criSocket = constants.ContainerRuntimeDockerSocket
	case constants.ContainerRuntimeCRIO:
		criSocket = constants.ContainerRuntimeCRIOSocket
	}

	aux := struct {
		userdata.UserData
		CRISocket string
	}{
		data,
		criSocket,
	}

	tmpl, err := template.New("").Parse(configuration)
	if err != nil {
		return err
	}
	var buf []byte
	writer := bytes.NewBuffer(buf)
	err = tmpl.Execute(writer, aux)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile("/etc/kubernetes/kubeadm.yaml", writer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write kubeadm.yaml: %s", err.Error())
	}

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

func (p *Kubeadm) Condition(data userdata.UserData) func() (bool, error) {
	switch data.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		return conditions.WaitForFileExists(constants.ContainerRuntimeDockerSocket)
	case constants.ContainerRuntimeCRIO:
		return conditions.WaitForFileExists(constants.ContainerRuntimeCRIOSocket)
	default:
		return conditions.None()
	}
}

func (p *Kubeadm) Env() []string { return []string{} }

func (p *Kubeadm) Type() Type { return Once }
