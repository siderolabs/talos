// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"errors"

	"github.com/siderolabs/gen/xslices"

	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// PatchDocument function is a helper which takes a config container, and patches the document with the given kind.
//
// It returns a copy of the container with the respective document kind patches.
//
// TODO: This should be a generic method on the container, but Go 1.26 does not support generic methods yet.
func PatchDocument[D config.Document](container coreconfig.Provider, patcher func(D) error) (coreconfig.Provider, error) {
	in := container.Documents()

	var errs error

	out := xslices.Map(in, func(doc config.Document) config.Document {
		d, ok := doc.(D)
		if !ok {
			return doc
		}

		clonedD := d.Clone()

		d, ok = clonedD.(D)
		if !ok {
			panic("cloned document is not of the expected type")
		}

		if err := patcher(d); err != nil {
			errs = errors.Join(errs, err)

			return doc
		}

		return d
	})

	if errs != nil {
		return nil, errs
	}

	return New(out...)
}
