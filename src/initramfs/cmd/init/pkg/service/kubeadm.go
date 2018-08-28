package service

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/src/initramfs/cmd/rotd/proto"
	"github.com/autonomy/dianemo/src/initramfs/pkg/crypto/x509"
	"github.com/autonomy/dianemo/src/initramfs/pkg/grpc/middleware/auth/basic"
	"github.com/autonomy/dianemo/src/initramfs/pkg/net"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
	"google.golang.org/grpc"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

const kubeadmSH = `#!/bin/bash

set -eou pipefail

cd /etc/kubernetes

{{- if .Init }}
{{- if eq .Init.Type "initial" }}
kubeadm init --config=kubeadm-config.yaml --ignore-preflight-errors=cri --skip-token-print
{{- else if eq .Init.Type "dependent" }}
export KUBECONFIG=/etc/kubernetes/admin.conf
kubeadm alpha phase certs all --config kubeadm-config.yaml
kubeadm alpha phase kubelet config write-to-disk --config kubeadm-config.yaml
kubeadm alpha phase kubelet write-env-file --config kubeadm-config.yaml
kubeadm alpha phase kubeconfig kubelet --config kubeadm-config.yaml
# Workaround for clusters running with the DenyEscalatingExec admission controller.
docker run \
	--rm \
	--net=host \
	--volume /etc/kubernetes/pki/etcd:/etc/kubernetes/pki/etcd k8s.gcr.io/etcd:{{ .EtcdVersion }} \
	etcdctl --ca-file /etc/kubernetes/pki/etcd/ca.crt --cert-file /etc/kubernetes/pki/etcd/peer.crt --key-file /etc/kubernetes/pki/etcd/peer.key --endpoints=https://{{ .Init.EtcdEndpoint }}:2379 member add {{ .Init.EtcdMemberName }} https://{{ .IP }}:2380
kubeadm alpha phase etcd local --config kubeadm-config.yaml
kubeadm alpha phase kubeconfig all --config kubeadm-config.yaml
{{- if not .Init.SelfHosted }}
	kubeadm alpha phase controlplane all --config kubeadm-config.yaml
{{- end }}
kubeadm alpha phase mark-master --config kubeadm-config.yaml
echo "successfully joined master node {{ .Hostname }}"
{{- end }}
{{- else }}
kubeadm join --config=kubeadm-config.yaml --ignore-preflight-errors=cri
{{- end }}
`

// Kubeadm implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubeadm struct{}

// Pre implements the Service interface.
func (p *Kubeadm) Pre(data userdata.UserData) (err error) {
	if data.Services.Kubeadm.Init != nil {
		if err = writeKubeadmPKIFiles(data.Security.Kubernetes.CA); err != nil {
			return
		}
	}

	if err = writeKubeadmConfig(data.Services.Kubeadm.Configuration); err != nil {
		return
	}

	return nil
}

// Post implements the Service interface.
func (p *Kubeadm) Post(data userdata.UserData) (err error) {
	if data.Services.Kubeadm.Init != nil && data.Services.Kubeadm.Init.TrustEndpoint == "" {
		return nil
	}

	creds := basic.NewCredentials(
		data.Security.OS.CA.Crt,
		data.Services.ROTD.Username,
		data.Services.ROTD.Password,
	)

	var conn *grpc.ClientConn
	parts := strings.Split(data.Services.Kubeadm.Init.TrustEndpoint, ":")
	if len(parts) != 2 {
		return fmt.Errorf("trust endpoint is not valid")
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	conn, err = basic.NewConnection(parts[0], i, creds)
	if err != nil {
		return
	}
	client := proto.NewROTDClient(conn)

	files := []string{
		"/etc/kubernetes/pki/ca.crt",
		"/etc/kubernetes/pki/ca.key",
		"/etc/kubernetes/pki/sa.key",
		"/etc/kubernetes/pki/sa.pub",
		"/etc/kubernetes/pki/front-proxy-ca.crt",
		"/etc/kubernetes/pki/front-proxy-ca.key",
		"/etc/kubernetes/pki/etcd/ca.crt",
		"/etc/kubernetes/pki/etcd/ca.key",
		"/etc/kubernetes/admin.conf",
	}
	if err = writeFiles(client, files); err != nil {
		return
	}

	return nil
}

// Cmd implements the Service interface.
func (p *Kubeadm) Cmd(data userdata.UserData, cmdArgs *CmdArgs) error {
	cmdArgs.Name = "kubeadm"
	cmdArgs.Path = "/bin/docker"
	cmdArgs.Args = []string{
		"run",
		"--rm",
		"--net=host",
		"--pid=host",
		"--privileged",
		"--volume=/sys:/sys:rw",
		"--volume=/sys/fs/cgroup:/sys/fs/cgroup:rw",
		"--volume=/var/run:/var/run:rw",
		"--volume=/run:/run:rw",
		"--volume=/var/lib/docker:/var/lib/docker:rw",
		"--volume=/var/lib/kubelet:/var/lib/kubelet:slave",
		"--volume=/var/log:/var/log",
		"--volume=/etc/kubernetes:/etc/kubernetes:shared",
		"--volume=/etc/os-release:/etc/os-release:ro",
		"--volume=/lib/modules:/lib/modules:ro",
		"--volume=/bin/docker:/bin/docker:ro",
		"--volume=/bin/crictl:/bin/crictl:ro",
		"--volume=/bin/kubeadm:/bin/kubeadm:ro",
		"--volume=/run/kubeadm.sh:/bin/kubeadm.sh:ro",
		"--name=kubeadm",
		"gcr.io/google_containers/hyperkube:v1.11.2",
		"/bin/kubeadm.sh",
	}

	contents, err := parse(data)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile("/run/kubeadm.sh", contents, os.FileMode(0700))

	return err
}

// Condition implements the Service interface.
func (p *Kubeadm) Condition(data userdata.UserData) func() (bool, error) {
	switch data.Services.Kubeadm.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		if data.Services.Kubeadm.Init != nil && data.Services.Kubeadm.Init.Type == "dependent" {
			return conditions.WaitForFilesToExist(constants.ContainerRuntimeDockerSocket, "/etc/kubernetes/admin.conf")
		}
		return conditions.WaitForFileExists(constants.ContainerRuntimeDockerSocket)
	case constants.ContainerRuntimeCRIO:
		if data.Services.Kubeadm.Init != nil && data.Services.Kubeadm.Init.Type == "dependent" {
			return conditions.WaitForFilesToExist(constants.ContainerRuntimeCRIOSocket, "/etc/kubernetes/admin.conf")
		}
		return conditions.WaitForFileExists(constants.ContainerRuntimeCRIOSocket)
	default:
		return conditions.None()
	}
}

// Env implements the Service interface.
func (p *Kubeadm) Env() []string { return []string{} }

// Type implements the Service interface.
func (p *Kubeadm) Type() Type { return Once }

func writeKubeadmConfig(data string) (err error) {
	if err = ioutil.WriteFile(constants.KubeadmConfig, []byte(data), 0400); err != nil {
		return fmt.Errorf("write %s: %s", constants.KubeadmConfig, err.Error())
	}

	return nil
}

func writeKubeadmPKIFiles(data *x509.PEMEncodedCertificateAndKey) (err error) {
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

func parse(data userdata.UserData) ([]byte, error) {
	ip, err := net.IP()
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	aux := struct {
		IP          string
		Hostname    string
		Init        *userdata.InitConfiguration
		EtcdVersion string
	}{
		Hostname:    hostname,
		IP:          ip.String(),
		Init:        data.Services.Kubeadm.Init,
		EtcdVersion: kubeadmconstants.DefaultEtcdVersion,
	}
	t, err := template.New("kubeadm").Parse(kubeadmSH)
	if err != nil {
		return nil, err
	}

	b := []byte{}
	buf := bytes.NewBuffer(b)
	err = t.Execute(buf, aux)

	return buf.Bytes(), err
}

func writeFiles(client proto.ROTDClient, files []string) (err error) {
	errChan := make(chan error)
	doneChan := make(chan bool)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelFunc()

	go func() {
		<-ctx.Done()
		errChan <- ctx.Err()
	}()

	go func() {
		for _, f := range files {
		L:
			b, err := ioutil.ReadFile(f)
			if err != nil {
				log.Printf("failed to read file %s: %v", f, err)
				time.Sleep(1 * time.Second)
				goto L
			}
			req := &proto.WriteFileRequest{
				Path: f,
				Data: b,
			}
			_, err = client.WriteFile(context.Background(), req)
			if err != nil {
				log.Printf("failed to write file %s: %v", f, err)
				time.Sleep(1 * time.Second)
				goto L
			}
		}
		doneChan <- true
	}()

	select {
	case err := <-errChan:
		return err
	case <-doneChan:
		return nil
	}
}
