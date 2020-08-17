// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dashboard implements simple UI for Talos cluster monitoring.
package dashboard

import (
	"context"
	"time"

	"github.com/talos-systems/talos/pkg/machinery/client"
)

// Main is the entrypoint into talosctl dashboard command.
func Main(ctx context.Context, c *client.Client, interval time.Duration) error {
	ui := &UI{}

	source := &APISource{
		Client:   c,
		Interval: interval,
	}

	dataCh := source.Run(ctx)
	defer source.Stop()

	return ui.Main(ctx, dataCh)
}
