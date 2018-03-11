// +build linux

package main

import "C"

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/mount"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/services"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/switchroot"
)

const (
	PATH = "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin:/opt/cni/bin"
)

var (
	switchRoot *bool
)

func parseProcCmdline() (cmdline map[string]string, err error) {
	cmdline = map[string]string{}
	cmdlineBytes, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return
	}
	arguments := strings.Split(string(cmdlineBytes), " ")
	for _, a := range arguments {
		kv := strings.Split(a, "=")
		if len(kv) == 2 {
			cmdline[kv[0]] = kv[1]
		}
	}

	return cmdline, err
}

func depmod() error {
	cmd := exec.Command("/bin/depmod", []string{}...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	os.Setenv("PATH", PATH)

	switchRoot = flag.Bool("switch-root", false, "perform a switch_root")
	flag.Parse()
}

func hang(errs []error) {
	for _, err := range errs {
		log.Println(err.Error())
	}

	// Hang infinitely to avoid a kernel panic.
	select {}
}

// TODO: Errors API that admins can use to debug.
func main() {
	if !*switchRoot {
		// Mount the initial file systems.
		if err := mount.Init(); err != nil {
			hang([]error{err})
		}
		// TODO: Execute user data.
		// Move the initial file systems to the new root.
		if err := mount.Move(); err != nil {
			hang([]error{err})
		}
		// Perform the equivalent of switch_root.
		// See https://github.com/karelzak/util-linux/blob/master/sys-utils/switch_root.c
		if err := switchroot.Switch(); err != nil {
			hang([]error{err})
		}
	}

	// Load the kernel modules.
	if err := depmod(); err != nil {
		hang([]error{err})
	}

	services.Start()

	fs := http.FileServer(http.Dir("/etc/kubernetes"))
	http.Handle("/", fs)
	http.ListenAndServe(":8080", nil)
}
