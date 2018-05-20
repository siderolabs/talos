package rootfs

import (
	"io/ioutil"
	"net"
	"os"
	"path"

	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/rootfs/etc"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/rootfs/proc"
	"github.com/autonomy/dianemo/initramfs/pkg/userdata"
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
func Prepare(s string, userdata userdata.UserData) (err error) {
	// Enable IP forwarding.
	if err = proc.WriteSystemProperty("net.ipv4.ip_forward", "1"); err != nil {
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
	// Save the user data to disk.
	data, err := yaml.Marshal(&userdata)
	if err != nil {
		return
	}
	if err = ioutil.WriteFile(path.Join(constants.NewRoot, constants.UserDataPath), data, 0400); err != nil {
		return
	}

	return nil
}
