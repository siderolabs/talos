package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/initramfs/pkg/userdata"
)

// Kubeadm implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubeadm struct{}

// Pre implements the Service interface.
func (p *Kubeadm) Pre(data userdata.UserData) (err error) {
	if data.Kubernetes.Init {
		if err = writeKubeadmPKIFiles(data.Kubernetes.CA); err != nil {
			return
		}
	}

	if err = writeKubeadmManifest(data.Kubernetes.Configuration); err != nil {
		return
	}

	return nil
}

// Cmd implements the Service interface.
func (p *Kubeadm) Cmd(data userdata.UserData) (name string, args []string) {
	var cmd string
	if data.Kubernetes.Init {
		cmd = "init"
	} else {
		cmd = "join"
	}
	name = "/bin/kubeadm"
	args = []string{
		cmd,
		"--config=/etc/kubernetes/kubeadm.yaml",
		"--ignore-preflight-errors=cri",
	}
	if data.Kubernetes.Init {
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

func writeKubeadmManifest(data string) (err error) {
	if err = ioutil.WriteFile(constants.KubeadmConfig, []byte(data), 0400); err != nil {
		return fmt.Errorf("write %s: %s", constants.KubeadmConfig, err.Error())
	}

	return nil
}

func writeKubeadmPKIFiles(data *userdata.PEMEncodedCertificateAndKey) (err error) {
	if err = os.MkdirAll(path.Dir(constants.KubeadmCACert), 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(constants.KubeadmCACert, data.Crt, 0400); err != nil {
		return fmt.Errorf("write %s: %s", constants.KubeadmCACert, err.Error())
	}

	if err = os.MkdirAll(path.Dir(constants.KubeadmCAKey), 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(constants.KubeadmCAKey, data.Key, 0400); err != nil {
		return fmt.Errorf("write %s: %s", constants.KubeadmCAKey, err.Error())
	}

	return nil
}
