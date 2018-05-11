package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/userdata"
)

// MasterConfiguration is the kubeadm manifest for master nodes.
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

// NodeConfiguration is the kubeadm manifest for worker nodes.
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

// Kubeadm implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubeadm struct{}

// Pre implements the Service interface.
func (p *Kubeadm) Pre(data userdata.UserData) (err error) {
	var configuration string
	if data.Kubernetes.Join {
		configuration = NodeConfiguration
	} else {
		configuration = MasterConfiguration
	}

	var socket string
	switch data.Kubernetes.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		socket = constants.ContainerRuntimeDockerSocket
	case constants.ContainerRuntimeCRIO:
		socket = constants.ContainerRuntimeCRIOSocket
	}

	if err = writeKubeadmManifest(data.Kubernetes, configuration, socket); err != nil {
		return
	}

	if !data.Kubernetes.Join {
		if err = writeKubeadmPKIFiles(data.Kubernetes); err != nil {
			return
		}
	}

	return nil
}

// Cmd implements the Service interface.
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
	if !data.Kubernetes.Join {
		args = append(args, "--skip-token-print")
	}

	return name, args
}

// Condition implements the Service interface.
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

// Env implements the Service interface.
func (p *Kubeadm) Env() []string { return []string{} }

// Type implements the Service interface.
func (p *Kubeadm) Type() Type { return Once }

func writeKubeadmManifest(data *userdata.Kubernetes, configuration, socket string) (err error) {
	aux := struct {
		*userdata.Kubernetes
		CRISocket string
	}{
		data,
		socket,
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

	if err = ioutil.WriteFile(constants.KubeadmConfig, writer.Bytes(), 0400); err != nil {
		return fmt.Errorf("write %s: %s", constants.KubeadmConfig, err.Error())
	}

	return nil
}

func writeKubeadmPKIFiles(data *userdata.Kubernetes) (err error) {
	caCrtBytes, err := base64.StdEncoding.DecodeString(data.CA.Crt)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(path.Dir(constants.KubeadmCACert), 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(constants.KubeadmCACert, caCrtBytes, 0400); err != nil {
		return fmt.Errorf("write %s: %s", constants.KubeadmCACert, err.Error())
	}

	caKeyBytes, err := base64.StdEncoding.DecodeString(data.CA.Key)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(path.Dir(constants.KubeadmCAKey), 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(constants.KubeadmCAKey, caKeyBytes, 0400); err != nil {
		return fmt.Errorf("write %s: %s", constants.KubeadmCAKey, err.Error())
	}

	return nil
}
