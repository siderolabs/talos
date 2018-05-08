package constants

const (
	// KernelParamRoot is the kernel parameter name for specifying the root
	// disk.
	KernelParamRoot = "dianemo.autonomy.io/root"

	// KernelParamUserData is the kernel parameter name for specifying the URL
	// to the user data.
	KernelParamUserData = "dianemo.autonomy.io/userdata"

	// NewRoot is the path where the switchroot target is mounted.
	NewRoot = "/root"

	// DataPartitionLabel is the label of the partition to use for mounting at
	// the data path.
	DataPartitionLabel = "DATA"

	// RootPartitionLabel is the label of the partition to use for mounting at
	// the root path.
	RootPartitionLabel = "ROOT"

	// PATH defines all locations where executables are stored.
	PATH = "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin:/opt/cni/bin"

	// ContainerRuntimeDocker is the name of the Docker container runtime.
	ContainerRuntimeDocker = "docker"

	// ContainerRuntimeDockerSocket is the path to the Docker daemon socket.
	ContainerRuntimeDockerSocket = "/var/run/docker.sock"

	// ContainerRuntimeCRIO is the name of the CRI-O container runtime.
	ContainerRuntimeCRIO = "crio"

	// ContainerRuntimeCRIOSocket is the path to the CRI-O daemon socket.
	ContainerRuntimeCRIOSocket = "/var/run/crio/crio.sock"

	// KubeadmConfig is the path to the kubeadm manifest file.
	KubeadmConfig = "/etc/kubernetes/kubeadm.yaml"

	// KubeadmCACert is the path to the root CA certificate.
	KubeadmCACert = "/etc/kubernetes/pki/ca.crt"

	// KubeadmCAKey is the path to the root CA private key.
	KubeadmCAKey = "/etc/kubernetes/pki/ca.key"
)

// See https://linux.die.net/man/3/klogctl
const (
	// SYSLOG_ACTION_SIZE_BUFFER is a named type argument to klogctl.
	// nolint: golint
	SYSLOG_ACTION_SIZE_BUFFER = 10

	// SYSLOG_ACTION_READ_ALL is a named type argument to klogctl.
	// nolint: golint
	SYSLOG_ACTION_READ_ALL = 3
)
