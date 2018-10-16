package constants

const (
	// KernelParamRoot is the kernel parameter name for specifying the root
	// disk.
	KernelParamRoot = "dianemo.autonomy.io/root"

	// KernelParamUserData is the kernel parameter name for specifying the URL
	// to the user data.
	KernelParamUserData = "dianemo.autonomy.io/userdata"

	// KernelParamPlatform is the kernel parameter name for specifying the
	// platform.
	KernelParamPlatform = "dianemo.autonomy.io/platform"

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

	// ContainerdSocket is the path to the containerd socket.
	ContainerdSocket = "/run/containerd/containerd.sock"

	// ContainerRuntimeDocker is the name of the Docker container runtime.
	ContainerRuntimeDocker = "docker"

	// ContainerRuntimeDockerSocket is the path to the Docker daemon socket.
	ContainerRuntimeDockerSocket = "/var/run/docker.sock"

	// ContainerRuntimeCRIO is the name of the CRI-O container runtime.
	ContainerRuntimeCRIO = "crio"

	// ContainerRuntimeCRIOSocket is the path to the CRI-O daemon socket.
	ContainerRuntimeCRIOSocket = "/var/run/crio/crio.sock"

	// KubeadmConfig is the path to the kubeadm manifest file.
	KubeadmConfig = "/var/etc/kubernetes/kubeadm-config.yaml"

	// KubeadmCACert is the path to the root CA certificate.
	KubeadmCACert = "/var/etc/kubernetes/pki/ca.crt"

	// KubeadmCAKey is the path to the root CA private key.
	KubeadmCAKey = "/var/etc/kubernetes/pki/ca.key"

	// KubeadmInitTypeInitial is the string that represents a master node that
	// is the initial node.
	KubeadmInitTypeInitial = "initial"

	// KubeadmInitTypeDependent is the string that represents a master node that
	// is not the initial node.
	KubeadmInitTypeDependent = "dependent"

	// KubernetesImage is the hyperkube image to use for the control plane.
	KubernetesImage = "gcr.io/google_containers/hyperkube:v1.13.0-alpha.1"

	// DockerImage is the docker image to use as the container runtime for
	// Kubernetes.
	DockerImage = "docker.io/library/docker:18.06.1-ce-dind"

	// CRIOImage is the cri-o image to use as the container runtime for
	// Kubernetes.
	CRIOImage = "docker.io/autonomy/cri-o:latest"

	// UserDataPath is the path to the downloaded user data.
	UserDataPath = "/var/run/userdata.yaml"
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
