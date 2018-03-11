package services

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"
)

const MasterConfiguration = `
kind: MasterConfiguration
apiVersion: kubeadm.k8s.io/v1alpha1
skipTokenPrint: true
networking:
  dnsDomain: cluster.local
  serviceSubnet: 10.96.0.0/12
  podSubnet: 10.244.0.0/16
featureGates:
  HighAvailability: true
  SelfHosting: true
  StoreCertsInSecrets: true
  DynamicKubeletConfig: true
`

func dockerd() {
	args := []string{
		"--live-restore",
		"--iptables=false",
		"--ip-masq=false",
		"--storage-driver=overlay2",
		"--selinux-enabled=false",
		"--exec-opt=native.cgroupdriver=cgroupfs",
		"--log-opt=max-size=10m",
		"--log-opt=max-file=3",
	}
	cmd := exec.Command("/bin/dockerd", args...)
	env := os.Environ()
	env = append(env, "DOCKER_RAMDISK=true")
	env = append(env, "DOCKER_NOFILE=1000000")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Println(err.Error())
	}
}

func kubeadm() {
	for {
		time.Sleep(1 * time.Second)
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {

		} else {
			break
		}
	}

	os.MkdirAll("/etc/kubernetes", os.ModeDir)
	err := ioutil.WriteFile("/etc/kubernetes/kubeadm.yaml", []byte(MasterConfiguration), 0644)
	if err != nil {
		log.Printf("failed to write kubeadm.yaml: %s", err.Error())
	}
	args := []string{
		"init",
		"--config=/etc/kubernetes/kubeadm.yaml",
	}
	cmd := exec.Command("/bin/kubeadm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		log.Println(err.Error())
	}
}

func kubelet() {
	for {
		time.Sleep(1 * time.Second)
		if _, err := os.Stat("/etc/kubernetes/kubelet.conf"); os.IsNotExist(err) {

		} else {
			break
		}
	}

	args := []string{
		"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf",
		"--kubeconfig=/etc/kubernetes/kubelet.conf",
		"--pod-manifest-path=/etc/kubernetes/manifests",
		"--allow-privileged=true",
		"--network-plugin=cni",
		"--cni-conf-dir=/etc/cni/net.d",
		"--cni-bin-dir=/opt/cni/bin",
		"--cluster-dns=10.96.0.10",
		"--cluster-domain=cluster.local",
		"--authorization-mode=Webhook",
		"--client-ca-file=/etc/kubernetes/pki/ca.crt",
		"--cgroup-driver=cgroupfs",
		"--cadvisor-port=0",
		"--rotate-certificates=true",
		"--serialize-image-pulls=false",
		"--feature-gates=ExperimentalCriticalPodAnnotation=true",
	}
	cmd := exec.Command("/bin/kubelet", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Println(err.Error())
	}
}

func cni() {
	os.Mkdir("/run/flannel", os.ModeDir)
	os.MkdirAll("/etc/cni/net.d", os.ModeDir)
}

func Start() {
	go cni()
	go dockerd()
	go kubeadm()
	go kubelet()
}
