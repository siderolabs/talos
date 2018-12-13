package rootfs

import (
	"io/ioutil"
	"net"
	"os"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/rootfs/cni"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/rootfs/etc"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/rootfs/proc"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
	yaml "gopkg.in/yaml.v2"
)

func ip() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}

// Prepare creates the files required by the installed binaries and libraries.
func Prepare(s string, data userdata.UserData) (err error) {
	// Enable IP forwarding.
	if err = proc.WriteSystemProperty(&proc.SystemProperty{Key: "net.ipv4.ip_forward", Value: "1"}); err != nil {
		return
	}
	// Kernel Self Protection Project recommended settings.
	if err = kernelHardening(); err != nil {
		return
	}
	// Create /etc/hosts.
	hostname, err := os.Hostname()
	if err != nil {
		return
	}
	ip := ip()
	if err = etc.Hosts(s, hostname, ip); err != nil {
		return
	}
	// Create /etc/resolv.conf.
	if err = etc.ResolvConf(s); err != nil {
		return
	}
	// Create /etc/os-release.
	if err = etc.OSRelease(s); err != nil {
		return
	}
	// Setup directories required by the CNI plugin.
	if err = cni.Setup(s, &data); err != nil {
		return
	}
	// Save the user data to disk.
	dataBytes, err := yaml.Marshal(&data)
	if err != nil {
		return
	}
	if err = ioutil.WriteFile(constants.UserDataPath, dataBytes, 0400); err != nil {
		return
	}

	return nil
}

// We can ignore setting kernel.kexec_load_disabled = 1 because modules are
// disabled in the kernel config.
func kernelHardening() (err error) {
	props := []*proc.SystemProperty{
		{
			Key:   "kernel.kptr_restrict",
			Value: "1",
		},
		{
			Key:   "kernel.dmesg_restrict",
			Value: "1",
		},
		{
			Key:   "kernel.perf_event_paranoid",
			Value: "3",
		},
		// {
		// 	Key:   "kernel.kexec_load_disabled",
		// 	Value: "1",
		// },
		{
			Key:   "kernel.yama.ptrace_scope",
			Value: "1",
		},
		{
			Key:   "user.max_user_namespaces",
			Value: "0",
		},
		// {
		// 	Key:   "kernel.unprivileged_bpf_disabled",
		// 	Value: "1",
		// },
		// {
		// 	Key:   "net.core.bpf_jit_harden",
		// 	Value: "2",
		// },
	}

	for _, prop := range props {
		if err = proc.WriteSystemProperty(prop); err != nil {
			return
		}
	}

	return nil
}
