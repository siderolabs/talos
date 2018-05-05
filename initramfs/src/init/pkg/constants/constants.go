package constants

const (
	KernelRootFlag               = "dianemo.autonomy.io/root"
	UserDataURLFlag              = "dianemo.autonomy.io/userdata"
	NewRoot                      = "/root"
	DATALabel                    = "DATA"
	ROOTLabel                    = "ROOT"
	PATH                         = "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin:/opt/cni/bin"
	ContainerRuntimeDocker       = "docker"
	ContainerRuntimeDockerSocket = "/var/run/docker.sock"
	ContainerRuntimeCRIO         = "crio"
	ContainerRuntimeCRIOSocket   = "/var/run/crio/crio.sock"
	KubeadmConfig                = "/etc/kubernetes/kubeadm.yaml"
	KubeadmCACert                = "/etc/kubernetes/pki/ca.crt"
	KubeadmCAKey                 = "/etc/kubernetes/pki/ca.key"
	SYSLOG_ACTION_SIZE_BUFFER    = 10
	SYSLOG_ACTION_READ_ALL       = 3
)
