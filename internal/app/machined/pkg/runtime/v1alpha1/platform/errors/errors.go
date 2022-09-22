// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package errors contains errors used by the platform package.
package errors

import "errors"

// ErrNoConfigSource indicates that the platform does not have a configured source for the configuration.
var ErrNoConfigSource = errors.New("no configuration source")

// ErrNoHostname indicates that the meta server does not have a instance hostname.
var ErrNoHostname = errors.New("failed to fetch hostname from metadata service")

// ErrNoExternalIPs indicates that the meta server does not have a external addresses.
var ErrNoExternalIPs = errors.New("failed to fetch external addresses from metadata service")

// ErrNoEventURL indicates that the platform does not have an expected events URL in the kernel params.
var ErrNoEventURL = errors.New("no event URL")

// ErrMetadataNotReady indicates that the platform does not have metadata yet.
var ErrMetadataNotReady = errors.New("platform metadata is not ready")
