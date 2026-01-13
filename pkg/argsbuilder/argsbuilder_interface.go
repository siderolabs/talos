// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package argsbuilder

// MergePolicy defines args builder args merging policy.
type MergePolicy int

const (
	// MergeOverwrite overwrite arg when merging.
	MergeOverwrite = iota
	// MergeAdditive concat argument lists.
	MergeAdditive
	// MergeDenied fail merge if another object has the arg defined.
	MergeDenied
)

// MergePolicies merge policy map.
type MergePolicies map[string]MergePolicy

// MergeOptions provides optional arguments for merge.
type MergeOptions struct {
	Policies MergePolicies
}

// MergeOption optional merge argument setter.
type MergeOption func(*MergeOptions)

// WithMergePolicies set merge policies during merge.
func WithMergePolicies(policies MergePolicies) MergeOption {
	return func(o *MergeOptions) {
		o.Policies = policies
	}
}

// WithDenyList disable merge for all keys in map.
func WithDenyList(denyList Args) MergeOption {
	return func(o *MergeOptions) {
		if o.Policies == nil {
			o.Policies = MergePolicies{}
		}

		for k := range denyList {
			o.Policies[k] = MergeDenied
		}
	}
}

// ArgsBuilder defines the requirements to build and manage a set of args.
type ArgsBuilder interface {
	MustMerge(Args, ...MergeOption)
	Merge(Args, ...MergeOption) error
	Set(string, []string) ArgsBuilder
	Args() []string
}
