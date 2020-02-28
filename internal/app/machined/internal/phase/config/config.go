// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

// Task represents the Task task.
type Task struct {
	cfgBytes *[]byte
}

// NewConfigTask initializes and returns a Task task.
func NewConfigTask(cfgBytes *[]byte) phase.Task {
	return &Task{
		cfgBytes: cfgBytes,
	}
}

// TaskFunc returns the runtime function.
func (task *Task) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *Task) standard(r runtime.Runtime) (err error) {
	cfg, err := config.NewFromFile(constants.ConfigPath)
	if err != nil || !cfg.Persist() {
		log.Printf("failed to read config from file or persistence disabled. re-pulling config")

		var cfgBytes []byte

		cfgBytes, err = fetchConfig(r)
		if err != nil {
			return err
		}

		*task.cfgBytes = cfgBytes

		return nil
	}

	log.Printf("using existing config on disk")

	cfgBytes, err := cfg.Bytes()
	if err != nil {
		return err
	}

	*task.cfgBytes = cfgBytes

	return nil
}

func fetchConfig(r runtime.Runtime) (out []byte, err error) {
	var b []byte

	if b, err = r.Platform().Configuration(); err != nil {
		return nil, err
	}

	// Detect if config is a gzip archive and unzip it if so
	contentType := http.DetectContentType(b)
	if contentType == "application/x-gzip" {
		var gzipReader *gzip.Reader

		gzipReader, err = gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("error creating gzip reader: %w", err)
		}

		// nolint: errcheck
		defer gzipReader.Close()

		var unzippedData []byte

		unzippedData, err = ioutil.ReadAll(gzipReader)
		if err != nil {
			return nil, fmt.Errorf("error unzipping machine config: %w", err)
		}

		b = unzippedData
	}

	return b, nil
}
