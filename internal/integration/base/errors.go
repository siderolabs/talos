// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package base

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IgnoreGRPCUnavailable searches through unwrapped errors and ignores the error if it is grpc.Unavailable.
func IgnoreGRPCUnavailable(err error) error {
	if err == nil {
		return nil
	}

	unwrappedErr := err

	for {
		if s, ok := status.FromError(unwrappedErr); ok && s.Code() == codes.Unavailable {
			// ignore errors if reboot happens before response is fully received
			return nil
		}

		unwrappedErr = errors.Unwrap(unwrappedErr)
		if unwrappedErr == nil {
			break
		}
	}

	return err
}
