// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kernel

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

const (
	// Sysfs defines prefix for sysfs kernel params.
	Sysfs = "sys"
	// Sysctl defines prefix for sysctl kernel params.
	Sysctl = "proc.sys"
)

// DefaultArgs returns the Talos default kernel commandline options.
func DefaultArgs(quirks quirks.Quirks) []string {
	result := []string{
		"init_on_alloc=1",
		"slab_nomerge=",
		"pti=on",
		"consoleblank=0",
		// AWS recommends setting the nvme_core.io_timeout to the highest value possible.
		// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nvme-ebs-volumes.html.
		"nvme_core.io_timeout=4294967295",
		// Disable rate limited printk
		"printk.devkmsg=on",
	}

	if quirks.SupportsIMA() {
		// Enable IMAs for integrity measurement
		result = append(
			result,
			"ima_template=ima-ng",
			"ima_appraise=fix",
			"ima_hash=sha512",
		)
	}

	if quirks.SupportsSELinux() {
		result = append(result, constants.KernelParamSELinux+"=1")
	}

	return result
}

// SecureBootArgs returns the kernel commandline options required for secure boot.
func SecureBootArgs(quirks.Quirks) []string {
	return []string{
		"lockdown=confidentiality",
	}
}

// Param represents a kernel system property.
type Param struct {
	Key   string
	Value string
}

// Path returns the path to the systctl file under /proc/sys or /sys.
func (prop *Param) Path() string {
	// From: https://man7.org/linux/man-pages/man5/sysctl.d.5.html
	//
	// Note that either "/" or "."  may be used as separators within
	// sysctl variable names. If the first separator is a slash,
	// remaining slashes and dots are left intact. If the first
	// separator is a dot, dots and slashes are interchanged.
	// "kernel.domainname=foo" and "kernel/domainname=foo" are
	// equivalent and will cause "foo" to be written to
	// /proc/sys/kernel/domainname. Either
	// "net.ipv4.conf.enp3s0/200.forwarding" or
	// "net/ipv4/conf/enp3s0.200/forwarding" may be used to refer to
	// /proc/sys/net/ipv4/conf/enp3s0.200/forwarding
	//
	// detect the first separator, either '.' or '/'
	// according to the sysctl man page, if the first separator is '/', we keep slashes intact,
	// otherwise we convert dots to slashes
	keyPath := prop.Key
	prefix := ""

	// trim standard prefix
	for _, stdPrefix := range []string{Sysctl, Sysfs} {
		if strings.HasPrefix(prop.Key, stdPrefix+".") {
			keyPath = keyPath[len(stdPrefix)+1:]
			prefix = stdPrefix

			break
		}
	}

	firstSepIndex := strings.IndexAny(keyPath, "./")
	// if the first separator is a dot, remap '.' to '/', and '/' to '.'
	if firstSepIndex != -1 && keyPath[firstSepIndex] == '.' {
		keyPath = strings.Map(
			func(r rune) rune {
				switch r {
				case '.':
					return '/'
				case '/':
					return '.'
				default:
					return r
				}
			},
			keyPath,
		)
	}

	return path.Clean("/" + filepath.Join(strings.ReplaceAll(prefix, ".", "/"), keyPath))
}
