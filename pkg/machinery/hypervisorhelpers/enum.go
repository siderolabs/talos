// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

import "slices"

// UnknownName is the name of the zero member every enum in this package carries.
//
// The zero member exists so that the generated protobuf has one at 0, as proto3 requires. It
// stands for a field that was never set, which is why it is not documented as a value a field
// accepts, even though the generated text methods will decode it.
const UnknownName = "unknown"

// NameableValues returns the members of an enum a document may name: every member but the zero
// value.
//
// Take the input from the generated `<Type>Strings()` helper rather than a hand-written list, so
// that adding a member cannot leave a stale set behind.
func NameableValues(values []string) []string {
	return slices.DeleteFunc(slices.Clone(values), func(name string) bool { return name == UnknownName })
}
