// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kernel

import (
	"path/filepath"
	"strings"
)

const (
	// Sysfs defines prefix for sysfs kernel params.
	Sysfs = "/sys"
	// Sysctl defines prefix for sysctl kernel params.
	Sysctl = "/proc/sys"
)

// DefaultArgs returns the Talos default kernel commandline options.
var DefaultArgs = []string{
	"init_on_alloc=1",
	"slab_nomerge=",
	"pti=on",
	"consoleblank=0",
	// AWS recommends setting the nvme_core.io_timeout to the highest value possible.
	// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nvme-ebs-volumes.html.
	"nvme_core.io_timeout=4294967295",
	"random.trust_cpu=on",
	// Disable rate limited printk
	"printk.devkmsg=on",
	"ima_template=ima-ng",
	"ima_appraise=fix",
	"ima_hash=sha512",
}

// Param represents a kernel system property.
type Param struct {
	Key   string
	Value string
}

// Path returns the path to the systctl file under /proc/sys.
func (prop *Param) Path() string {
	res := strings.ReplaceAll(prop.Key, ".", "/")

	// fallback to the old behavior if the key path is not absolute
	if !strings.HasPrefix(res, "/") {
		res = filepath.Join(Sysctl, res)
	}

	return res
}
