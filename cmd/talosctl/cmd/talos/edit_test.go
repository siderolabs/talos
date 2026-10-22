// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package talos //nolint:testpackage // to test editFn

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
)

const (
	editTestConfig        = "version: v1alpha1\nmachine:\n  type: worker\n"
	editTestInvalidConfig = "version: [\n"
	editTestValidConfig   = "version: v1alpha1\nmachine:\n  type: controlplane\n"
)

type editMachineClient struct {
	machine.MachineServiceClient
}

func (editMachineClient) ApplyConfiguration(_ context.Context, req *machine.ApplyConfigurationRequest, _ ...grpc.CallOption) (*machine.ApplyConfigurationResponse, error) {
	_, err := configloader.NewFromBytes(req.Data)
	if err != nil {
		return nil, err
	}

	return &machine.ApplyConfigurationResponse{}, nil
}

// installEditor sets up a shell script as the editor for the test.
//
// The script runs the given shell snippet with $1 set to the file being edited and $N set to the
// 1-based number of the editor launch, and the returned function lists the files edited so far.
//
// Temporary files are redirected into the test directory, so they are cleaned up with it.
func installEditor(t *testing.T, script string) func() []string {
	t.Helper()

	dir := t.TempDir()
	pathsFile := filepath.Join(dir, "paths")
	editorPath := filepath.Join(dir, "editor.sh")

	require.NoError(t, os.WriteFile(editorPath, []byte(
		"#!/bin/sh\nprintf '%s\\n' \"$1\" >> \"$EDIT_PATHS\"\nN=$(wc -l < \"$EDIT_PATHS\" | tr -d ' ')\n"+script,
	), 0o700))

	t.Setenv("TMPDIR", dir)
	t.Setenv("TALOS_EDITOR", editorPath)
	t.Setenv("EDIT_PATHS", pathsFile)

	return func() []string {
		paths, err := os.ReadFile(pathsFile)
		require.NoError(t, err)

		return strings.Split(strings.TrimSpace(string(paths)), "\n")
	}
}

func editTestResource(t *testing.T) *configres.MachineConfig {
	t.Helper()

	cfg, err := configloader.NewFromBytes([]byte(editTestConfig))
	require.NoError(t, err)

	return configres.NewMachineConfig(cfg)
}

func TestEditPreservesRejectedChanges(t *testing.T) {
	editedPaths := installEditor(t, `if [ "$N" -eq 1 ]; then printf '`+editTestInvalidConfig+`' > "$1"; fi`)

	err := editFn()(context.Background(), &client.Client{MachineClient: editMachineClient{}}, "127.0.0.1", editTestResource(t))
	require.ErrorContains(t, err, "Edit canceled, no valid changes were saved.")

	files := editedPaths()
	require.Len(t, files, 2)

	require.Contains(t, err.Error(), files[1])
	data, readErr := os.ReadFile(files[1])
	require.NoError(t, readErr, "the recovery copy named in the error must remain available")
	require.Contains(t, string(data), editTestInvalidConfig)
	require.NoFileExists(t, files[0], "superseded temporary files should still be removed")
}

func TestEditRetriesRejectedChanges(t *testing.T) {
	editedPaths := installEditor(t, `if [ "$N" -eq 1 ]; then printf '`+editTestInvalidConfig+`' > "$1"; else printf '`+editTestValidConfig+`' > "$1"; fi`)

	require.NoError(t, editFn()(context.Background(), &client.Client{MachineClient: editMachineClient{}}, "127.0.0.1", editTestResource(t)))

	files := editedPaths()
	require.Len(t, files, 2)

	for _, file := range files {
		require.NoFileExists(t, file)
	}
}

func TestEditResetsErrorBetweenNodes(t *testing.T) {
	editedPaths := installEditor(t, `if [ "$N" -le 2 ]; then printf '`+editTestInvalidConfig+`' > "$1"; fi`)

	fn := editFn()
	c := &client.Client{MachineClient: editMachineClient{}}

	err := fn(context.Background(), c, "127.0.0.1", editTestResource(t))
	require.ErrorContains(t, err, "Edit canceled, no valid changes were saved.")

	// the next node leaves the configuration unchanged, so the previous node's error must not cancel the edit
	require.NoError(t, fn(context.Background(), c, "127.0.0.2", editTestResource(t)))

	files := editedPaths()
	require.Len(t, files, 3)
	require.NoFileExists(t, files[2])
}

func TestEditRemovesTemporaryFiles(t *testing.T) {
	for _, test := range []struct {
		name        string
		edit        string
		expectedErr string
	}{
		{name: "unchanged"},
		{name: "accepted", edit: `printf '\n' >> "$1"`},
		{name: "empty", edit: `: > "$1"`},
		{name: "editor failed", edit: `exit 1`, expectedErr: "there was a problem with the editor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			editedPaths := installEditor(t, test.edit)

			err := editFn()(context.Background(), &client.Client{MachineClient: editMachineClient{}}, "127.0.0.1", editTestResource(t))
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}

			files := editedPaths()
			require.Len(t, files, 1)
			require.NoFileExists(t, files[0])
		})
	}
}
