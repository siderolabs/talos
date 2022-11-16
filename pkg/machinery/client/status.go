// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Status returns the status if it is a Status error, nil otherwise.
func Status(err error) *status.Status {
	type grpcStatus interface {
		GRPCStatus() *status.Status
	}

	// Don't use FromError to avoid allocation of OK status.
	var st grpcStatus

	if errors.As(err, &st) {
		return st.GRPCStatus()
	}

	return nil
}

// StatusCode returns the Code of the error if it is a Status error, codes.OK if err
// is nil, or codes.Unknown otherwise correctly unwrapping wrapped errors.
//
// StatusCode is mostly equivalent to grpc `status.Code` method, but it correctly unwraps wrapped errors
// including `multierror.Error` used when parsing multi-node responses.
func StatusCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}

	if st := Status(err); st != nil {
		return st.Code()
	}

	return codes.Unknown
}
