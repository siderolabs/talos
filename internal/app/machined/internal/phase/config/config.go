/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package config

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Task represents the Task task.
type Task struct{}

// NewConfigTask initializes and returns a Task task.
func NewConfigTask() phase.Task {
	return &Task{}
}

// TaskFunc returns the runtime function.
func (task *Task) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *Task) standard(r runtime.Runtime) (err error) {
	var b []byte

	if b, err = r.Platform().Configuration(); err != nil {
		return err
	}

	// Detect if config is a gzip archive and unzip it if so
	contentType := http.DetectContentType(b)
	if contentType == "application/x-gzip" {
		gzipReader, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("error creating gzip reader: %w", err)
		}

		// nolint: errcheck
		defer gzipReader.Close()

		unzippedData, err := ioutil.ReadAll(gzipReader)
		if err != nil {
			return fmt.Errorf("error unzipping machine config: %w", err)
		}

		b = unzippedData
	}

	return ioutil.WriteFile(constants.ConfigPath, b, 0600)
}
