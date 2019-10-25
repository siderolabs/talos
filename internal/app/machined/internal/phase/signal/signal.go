// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package signal

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/event"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// Handler represents the signal handler task.
type Handler struct{}

// NewHandlerTask initializes and returns a signal handler task.
func NewHandlerTask() phase.Task {
	return &Handler{}
}

// TaskFunc returns the runtime function.
func (task *Handler) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return task.container
	default:
		return nil
	}
}

func (task *Handler) container(r runtime.Runtime) (err error) {
	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, syscall.SIGTERM)

	go func() {
		<-termCh
		signal.Stop(termCh)

		log.Printf("shutdown via SIGTERM received")
		event.Bus().Notify(event.Event{Type: event.Shutdown})
	}()

	return nil
}
