// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"io"

	"k8s.io/klog/v2"
)

func init() {
	// Kubernetes client likes to do calls to `klog` in random places which are not configurable.
	// For Talos this means those logs are going to the console which doesn't look good.
	klog.EnableContextualLogging(false)
	klog.SetOutput(io.Discard)
	klog.LogToStderr(false)
}
