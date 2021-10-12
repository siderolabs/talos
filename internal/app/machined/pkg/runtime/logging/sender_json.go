// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

type jsonSender struct {
	l *log.Logger
}

// NewJSON returns log sender that would eventually send logs in JSON.
// FIXME(aleksi): update comment.
func NewJSON() runtime.LogSender {
	return &jsonSender{
		l: log.New(os.Stdout, "JSON: ", 0),
	}
}

func (j *jsonSender) Send(ctx context.Context, e *runtime.LogEvent) error {
	m := make(map[string]interface{}, len(e.Fields)+3)
	m["msg"] = e.Msg
	m["time"] = e.Time.Unix()
	m["level"] = e.Level.String()

	for k, v := range e.Fields {
		m[k] = v
	}

	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("%w: %s", runtime.ErrDontRetry, err)
	}

	j.l.Printf("%s\n", b)

	return nil
}
