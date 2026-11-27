// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"

	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/containerlogdata"
)

// consumeLogs drains a log source channel into write, called on the UI goroutine.
//
// Lines are batched so that a burst produces one queued update, as the dashboard does for node logs.
func consumeLogs(ctx context.Context, app *tview.Application, logCh <-chan containerlogdata.Data, write func(containerlogdata.Data)) {
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-logCh:
			batch := []containerlogdata.Data{data}

		drain:
			for {
				select {
				case more := <-logCh:
					batch = append(batch, more)
				default:
					break drain
				}
			}

			app.QueueUpdate(func() {
				for _, entry := range batch {
					write(entry)
				}
			})
		}
	}
}
