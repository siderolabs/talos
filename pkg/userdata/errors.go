/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import "errors"

var (
	// General
	ErrRequiredSection = errors.New("required userdata section")
	ErrInvalidVersion  = errors.New("invalid config version")

	// Security
	ErrInvalidCert     = errors.New("Certificate is invalid")
	ErrInvalidCertType = errors.New("Certificate type is invalid")

	// Services
	ErrUnsupportedCNI     = errors.New("unsupported CNI driver")
	ErrInvalidTrustdToken = errors.New("trustd token is invalid")

	// Networking
	ErrBadAddressing  = errors.New("invalid network device addressing method")
	ErrInvalidAddress = errors.New("invalid network address")
)
