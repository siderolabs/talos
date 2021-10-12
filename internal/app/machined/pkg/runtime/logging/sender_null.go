// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"context"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

type nullSender struct{}

// NewNull returns log sender that does nothing.
func NewNull() runtime.LogSender {
	return nullSender{}
}

func (nullSender) Send(context.Context, *runtime.LogEvent) error {
	return nil
}
