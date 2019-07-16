/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kspp

import (
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/kernel"
)

var (
	// RequiredKSPPKernelParameters is the set of kernel parameters required to
	// satisfy the KSPP.
	// # TODO(andrewrynhard): Add slub_debug=P. See https://github.com/talos-systems/talos/pull/157.
	RequiredKSPPKernelParameters = kernel.Parameters{
		kernel.NewParameter("page_poison").Append("1"),
		kernel.NewParameter("slab_nomerge").Append(""),
		kernel.NewParameter("pti").Append("on"),
	}
)

// EnforceKSPPKernelParameters verifies that all required KSPP kernel
// parameters are present with the right value.
func EnforceKSPPKernelParameters() error {
	var result *multierror.Error
	for _, values := range RequiredKSPPKernelParameters {
		var val *string
		if val = kernel.Cmdline().Get(values.Key()).First(); val == nil {
			result = multierror.Append(result, errors.Errorf("KSPP kernel parameter %s is required", values.Key()))
			continue
		}

		expected := values.First()
		if *val != *expected {
			result = multierror.Append(result, errors.Errorf("KSPP kernel parameter %s was found with value %s, expected %s", values.Key(), *val, *expected))
		}
	}

	return result.ErrorOrNil()
}
