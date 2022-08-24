// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package base

import "strings"

// StringList implements flag.Value for list of strings.
type StringList []string

// String implements flag.Value.
func (l *StringList) String() string {
	return strings.Join(*l, ",")
}

// Set implements flag.Value.
func (l *StringList) Set(value string) error {
	*l = append(*l, strings.Split(value, ",")...)

	return nil
}
