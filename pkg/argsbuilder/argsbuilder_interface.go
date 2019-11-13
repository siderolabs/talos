// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package argsbuilder

// ArgsBuilder defines the requirements to build and manage a set of args.
type ArgsBuilder interface {
	Merge(Args) ArgsBuilder
	Set(string, string) ArgsBuilder
	Args() []string
}
