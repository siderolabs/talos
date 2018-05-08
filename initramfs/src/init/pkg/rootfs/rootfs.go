package rootfs

import (
	"net"
	"os"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/etc"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
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
func Prepare(s string, userdata userdata.UserData) error {
	// Create /etc/hosts
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	ip := ip()
	if err := etc.Hosts(s, hostname, ip); err != nil {
		return err
	}
	// Create /etc/resolv.conf
	if err := etc.ResolvConf(s, userdata); err != nil {
		return err
	}

	return nil
}
