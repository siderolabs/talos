// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides controllers which manage Kubernetes resources.
package k8s

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func init() {
	// ugly hack, but it doesn't look like there's better API
	// cut out error handler which logs error to standard logger
	utilruntime.ErrorHandlers = utilruntime.ErrorHandlers[len(utilruntime.ErrorHandlers)-1:] //nolint:reassign
}
