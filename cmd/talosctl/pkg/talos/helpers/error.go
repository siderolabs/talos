// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/gertd/go-pluralize"
	"github.com/hashicorp/go-multierror"
)

// AppendErrors adds errors to the multierr wrapper.
func AppendErrors(err error, errs ...error) error {
	res := multierror.Append(err, errs...)

	res.ErrorFormat = func(errs []error) string {
		lines := make([]string, 0, len(errs))

		for _, err := range errs {
			lines = append(lines, fmt.Sprintf(" %s", err.Error()))
		}

		count := pluralize.NewClient().Pluralize("error", len(lines), true)

		return color.RedString(fmt.Sprintf("%s occurred:\n%s", count, strings.Join(lines, "\n")))
	}

	return res
}
