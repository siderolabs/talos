// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Code generated by "deep-copy -type ProcessorSpec -type MemorySpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go ."; DO NOT EDIT.

package hardware

// DeepCopy generates a deep copy of ProcessorSpec.
func (o ProcessorSpec) DeepCopy() ProcessorSpec {
	var cp ProcessorSpec = o
	return cp
}

// DeepCopy generates a deep copy of MemorySpec.
func (o MemorySpec) DeepCopy() MemorySpec {
	var cp MemorySpec = o
	return cp
}
