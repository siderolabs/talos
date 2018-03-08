package switchroot

import (
	"fmt"
	"os"
	"syscall"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	"golang.org/x/sys/unix"
)

func Switch() error {
	if err := unix.Chroot(constants.NewRoot); err != nil {
		return fmt.Errorf("failed to chroot: %s", err.Error())
	}
	if err := syscall.Exec("/sbin/init", []string{"init", "--switch-root"}, os.Environ()); err != nil {
		return fmt.Errorf("failed to exec /sbin/init: %s", err.Error())
	}

	return nil
}
