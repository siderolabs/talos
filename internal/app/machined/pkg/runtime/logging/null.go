// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"io"
	"os"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

// NullLoggingManager sends all the logs to /dev/null.
type NullLoggingManager struct{}

// NewNullLoggingManager initializes NullLoggingManager.
func NewNullLoggingManager() *NullLoggingManager {
	return &NullLoggingManager{}
}

// ServiceLog implements LoggingManager.
func (manager *NullLoggingManager) ServiceLog(id string) runtime.LogHandler {
	return &nullLogHandler{}
}

type nullLogHandler struct{}

func (handler *nullLogHandler) Writer() (io.WriteCloser, error) {
	return os.OpenFile(os.DevNull, os.O_WRONLY, 0)
}

func (handler *nullLogHandler) Reader(...runtime.LogOption) (io.ReadCloser, error) {
	return os.OpenFile(os.DevNull, os.O_RDONLY, 0)
}
