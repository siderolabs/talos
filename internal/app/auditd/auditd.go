// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package auditd registers auditd service and logs audit events.
package auditd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/elastic/go-libaudit/v2"
	"github.com/elastic/go-libaudit/v2/auparse"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// Main is an entrypoint to the auditd service.
func Main(ctx context.Context, _ runtime.Runtime, logWriter io.Writer) error {
	return Run(ctx, logWriter)
}

// Run starts the auditd service.
//
// based on https://github.com/elastic/go-libaudit/blob/main/cmd/audit/audit.go
func Run(ctx context.Context, logWriter io.Writer) error {
	var wg sync.WaitGroup

	defer wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	client, err := libaudit.NewAuditClient(nil)
	if err != nil {
		return fmt.Errorf("failed to create audit client: %w", err)
	}

	var auditDefaultEnabled atomic.Bool

	wg.Add(1)

	go func(c *libaudit.AuditClient) {
		defer wg.Done()

		<-ctx.Done()

		if !auditDefaultEnabled.Load() {
			c.SetEnabled(false, libaudit.NoWait) //nolint:errcheck
		}

		c.Close() //nolint:errcheck
	}(client)

	status, err := client.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get audit status: %w", err)
	}

	auditDefaultEnabled.Store(status.Enabled >= 1)

	if status.Enabled == 0 {
		if err := client.SetEnabled(true, libaudit.WaitForReply); err != nil {
			return fmt.Errorf("failed to enable audit: %w", err)
		}
	}

	if err = client.SetRateLimit(uint32(4096), libaudit.NoWait); err != nil {
		return fmt.Errorf("failed to set rate limit: %w", err)
	}

	if err := client.SetBacklogLimit(8192, libaudit.NoWait); err != nil {
		return fmt.Errorf("failed to set backlog limit: %w", err)
	}

	if err := client.SetPID(libaudit.NoWait); err != nil {
		return fmt.Errorf("failed to set audit PID: %w", err)
	}

	return receiveEvents(ctx, client, logWriter)
}

func receiveEvents(ctx context.Context, client *libaudit.AuditClient, logWriter io.Writer) error {
	for {
		rawEvent, err := client.Receive(false)
		if err != nil {
			if errors.Is(err, syscall.EBADF) {
				return nil
			}

			if errors.Is(err, syscall.EINTR) && errors.Is(err, syscall.EAGAIN) {
				continue
			}

			return fmt.Errorf("failed to receive audit event: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Messages from 1100-2999 are valid audit messages.
		if rawEvent.Type < auparse.AUDIT_USER_AUTH ||
			rawEvent.Type > auparse.AUDIT_LAST_USER_MSG2 {
			continue
		}

		fmt.Fprintf(logWriter, "type=%s msg=%s\n", rawEvent.Type, rawEvent.Data)
	}
}
