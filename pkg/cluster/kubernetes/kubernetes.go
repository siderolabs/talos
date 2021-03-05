// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubernetes provides cluster-wide kubernetes utilities.
package kubernetes

import (
	"errors"
	"net"
	"syscall"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func retryableError(err error) bool {
	if apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) || apierrors.IsInternalError(err) {
		return true
	}

	netErr := &net.OpError{}

	if errors.As(err, &netErr) {
		return netErr.Temporary() || errors.Is(netErr.Err, syscall.ECONNREFUSED)
	}

	return false
}
