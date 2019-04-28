/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kspp

import (
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/kernel"
)

// RequiredKSPPKernelParameters is the set of kernel parameters required to
// satisfy the KSPP.
// # TODO(andrewrynhard): Add slub_debug=P. See https://github.com/talos-systems/talos/pull/157.
var RequiredKSPPKernelParameters = map[string]string{"page_poison": "1", "slab_nomerge": "", "pti": "on"}

// EnforceKSPPKernelParameters verifies that all required KSPP kernel
// parameters are present with the right value.
func EnforceKSPPKernelParameters() error {
	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return err
	}

	var result *multierror.Error
	for param, expected := range RequiredKSPPKernelParameters {
		var (
			ok  bool
			val string
		)
		if val, ok = arguments[param]; !ok {
			result = multierror.Append(result, errors.Errorf("KSPP kernel parameter %s is required", param))
			continue
		}
		if val != expected {
			result = multierror.Append(result, errors.Errorf("KSPP kernel parameter %s was found with value %s, expected %s", param, val, expected))
		}
	}

	return result.ErrorOrNil()
}
