/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

type Version string

func (v Version) Validate() error {
	var result *multierror.Error
	if v == "" {
		result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "version", "", ErrRequiredSection))
	}

	if v != "1" {
		result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "version", v, ErrInvalidVersion))
	}

	return result.ErrorOrNil()
}
