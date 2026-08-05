// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package log

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// annotationsKey is used to store the request annotations in the context.
type annotationsKey struct{}

// annotations accumulates messages to be reported as part of the request log line.
type annotations struct {
	mu       sync.Mutex
	messages []string
}

func (a *annotations) add(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.messages = append(a.messages, message)
}

// String formats the collected messages, returning an empty string if there are none.
func (a *annotations) String() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.messages) == 0 {
		return ""
	}

	return " {" + strings.Join(a.messages, "; ") + "}"
}

// withAnnotations returns a context which collects annotations for the request.
func withAnnotations(ctx context.Context) (context.Context, *annotations) {
	a := &annotations{}

	return context.WithValue(ctx, annotationsKey{}, a), a
}

// Annotatef appends a message to the log line of the gRPC request being handled.
//
// Interceptors and handlers use it to report details (e.g. authorization decisions) which
// end up on the single log line the logging middleware emits once the request is complete.
//
// If the context doesn't carry the annotations (the server has no logging middleware installed),
// the message is dropped.
func Annotatef(ctx context.Context, format string, v ...any) {
	a, ok := ctx.Value(annotationsKey{}).(*annotations)
	if !ok {
		return
	}

	a.add(fmt.Sprintf(format, v...))
}
