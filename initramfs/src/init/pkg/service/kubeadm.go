package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
)

const MasterConfiguration = `
kind: MasterConfiguration
apiVersion: kubeadm.k8s.io/v1alpha1
kubernetesVersion: v1.10.2
token: {{ .Token }}
tokenTTL: 0s
criSocket: {{ .CRISocket }}
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
discoveryTokenAPIServers:
  - {{ .APIServer }}
discoveryTokenCACertHashes:
{{ range $_, $hash := .DiscoveryTokenCACertHashes }}
- {{ $hash }}
{{ end }}
criSocket: {{ .CRISocket }}
nodeName: {{ .NodeName }}
`

type Kubeadm struct{}

var cmd string

func (p *Kubeadm) Pre(data userdata.UserData) error {
	var configuration string
	if data.Kubernetes.Join {
		configuration = NodeConfiguration
	} else {
		configuration = MasterConfiguration
	}

	var criSocket string
	switch data.Kubernetes.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		criSocket = constants.ContainerRuntimeDockerSocket
	case constants.ContainerRuntimeCRIO:
		criSocket = constants.ContainerRuntimeCRIOSocket
	}

	aux := struct {
		*userdata.Kubernetes
		CRISocket string
	}{
		data.Kubernetes,
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

	if err := ioutil.WriteFile(constants.KubeadmConfig, writer.Bytes(), 0400); err != nil {
		return fmt.Errorf("write %s: %s", constants.KubeadmConfig, err.Error())
	}

	if !data.Kubernetes.Join {
		caCrtBytes, err := base64.StdEncoding.DecodeString(data.Kubernetes.CA.Crt)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path.Dir(constants.KubeadmCACert), 0600); err != nil {
			return err
		}
		if err := ioutil.WriteFile(constants.KubeadmCACert, caCrtBytes, 0400); err != nil {
			return fmt.Errorf("write %s: %s", constants.KubeadmCACert, err.Error())
		}
		caKeyBytes, err := base64.StdEncoding.DecodeString(data.Kubernetes.CA.Key)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path.Dir(constants.KubeadmCAKey), 0600); err != nil {
			return err
		}
		if err := ioutil.WriteFile(constants.KubeadmCAKey, caKeyBytes, 0400); err != nil {
			return fmt.Errorf("write %s: %s", constants.KubeadmCAKey, err.Error())
		}
	}

	return nil
}

func (p *Kubeadm) Cmd(data userdata.UserData) (name string, args []string) {
	var cmd string
	if data.Kubernetes.Join {
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
	switch data.Kubernetes.ContainerRuntime {
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
