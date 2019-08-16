/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package v1 provides user-facing v1 machine configs
// nolint: dupl
package v1

import "errors"

var (
	// General

	// ErrRequiredSection denotes a section is required
	ErrRequiredSection = errors.New("required userdata section")
	// ErrInvalidVersion denotes that the config file version is invalid
	ErrInvalidVersion = errors.New("invalid config version")

	// Security

	// ErrInvalidCert denotes that the certificate specified is invalid
	ErrInvalidCert = errors.New("certificate is invalid")
	// ErrInvalidCertType denotes that the certificate type is invalid
	ErrInvalidCertType = errors.New("certificate type is invalid")

	// Services

	// ErrUnsupportedCNI denotes that the specified CNI is invalid
	ErrUnsupportedCNI = errors.New("unsupported CNI driver")
	// ErrInvalidTrustdToken denotes that a trustd token has not been specified
	ErrInvalidTrustdToken = errors.New("trustd token is invalid")

	// Networking

	// ErrBadAddressing denotes that an incorrect combination of network
	// address methods have been specified
	ErrBadAddressing = errors.New("invalid network device addressing method")
	// ErrInvalidAddress denotes that a bad address was provided
	ErrInvalidAddress = errors.New("invalid network address")
)
