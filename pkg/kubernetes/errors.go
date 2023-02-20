// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"github.com/siderolabs/go-kubernetes/kubernetes"
)

// IsRetryableError returns true if this Kubernetes API should be retried.
func IsRetryableError(err error) bool {
	return kubernetes.IsRetryableError(err)
}
