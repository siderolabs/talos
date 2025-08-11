// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package etcd provides resources which interface with etcd.
package etcd

import (
	"fmt"
	"strconv"

	"github.com/cosi-project/runtime/pkg/resource"
)

//go:generate go tool github.com/siderolabs/deep-copy -type ConfigSpec -type PKIStatusSpec -type SpecSpec -type MemberSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName contains resources supporting etcd service.
const NamespaceName resource.Namespace = "etcd"

// FormatMemberID represents a uint64 in hexadecimal notation.
func FormatMemberID(memberID uint64) string {
	return fmt.Sprintf("%016x", memberID)
}

// ParseMemberID converts a member ID in hexadecimal notation to a uint64.
func ParseMemberID(memberID string) (uint64, error) {
	id, err := strconv.ParseUint(memberID, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing etcd member id: %w", err)
	}

	return id, nil
}
