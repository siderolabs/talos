// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lifecycle_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/lifecycle"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func TestProgressWriterFailure(t *testing.T) {
	writer := lifecycle.NewProgressWriter(true)

	for _, message := range []string{
		"machine configuration is invalid: unknown key\n",
		"Usage:\n",
		"  installer install [flags]\n",
	} {
		writer.UpdateJob("172.20.0.5", messageProgress(message))
	}

	writer.UpdateJob("172.20.0.6", messageProgress("output from another node\n"))
	writer.UpdateJob("172.20.0.5", &machine.LifecycleServiceInstallProgress{
		Response: &machine.LifecycleServiceInstallProgress_ExitCode{ExitCode: 1},
	})

	failure := writer.Failure("172.20.0.5", "upgrade", 1)

	assert.Equal(t, `172.20.0.5: installer output:
machine configuration is invalid: unknown key
Usage:
  installer install [flags]
172.20.0.5: upgrade failed with exit code 1`, failure)
	assert.NotContains(t, failure, "output from another node")
}

func TestProgressWriterOutputCapture(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		writer := lifecycle.NewProgressWriter(false)
		writer.UpdateJob("node", messageProgress("installer diagnostic\n"))

		assert.Equal(t, "node: upgrade failed with exit code 1", writer.Failure("node", "upgrade", 1))
	})

	t.Run("bounded", func(t *testing.T) {
		writer := lifecycle.NewProgressWriter(true)
		writer.UpdateJob("node", messageProgress("oldest output\n"))
		writer.UpdateJob("node", messageProgress(strings.Repeat("x", 70*1024)))

		failure := writer.Failure("node", "upgrade", 1)

		assert.Contains(t, failure, "[earlier installer output truncated]")
		assert.NotContains(t, failure, "oldest output")
		assert.LessOrEqual(t, len(failure), 66*1024)
	})
}

func TestProgressWriterFailureExitCodeClassification(t *testing.T) {
	writer := lifecycle.NewProgressWriter(false)

	for _, test := range []struct {
		exitCode int32
		expected string
	}{
		{exitCode: 1, expected: "node: upgrade failed with exit code 1"},
		{exitCode: 2, expected: "node: upgrade failed with exit code 2 (invalid input)"},
		{exitCode: 3, expected: "node: upgrade failed with exit code 3 (unsupported operation)"},
		{exitCode: 4, expected: "node: upgrade failed with exit code 4 (environment error)"},
		{exitCode: 5, expected: "node: upgrade failed with exit code 5 (dependency error)"},
		{exitCode: 6, expected: "node: upgrade failed with exit code 6 (I/O error)"},
		{exitCode: 7, expected: "node: upgrade failed with exit code 7 (installation error)"},
	} {
		t.Run(test.expected, func(t *testing.T) {
			assert.Equal(t, test.expected, writer.Failure("node", "upgrade", test.exitCode))
		})
	}
}

func messageProgress(message string) *machine.LifecycleServiceInstallProgress {
	return &machine.LifecycleServiceInstallProgress{
		Response: &machine.LifecycleServiceInstallProgress_Message{Message: message},
	}
}
