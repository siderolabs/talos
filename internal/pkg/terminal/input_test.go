// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package terminal_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/siderolabs/talos/internal/pkg/terminal"
)

type finalBytesInput struct {
	file  *os.File
	reads int
}

func (input *finalBytesInput) Fd() uintptr { return input.file.Fd() }

func (input *finalBytesInput) Read(buffer []byte) (int, error) {
	input.reads++

	switch input.reads {
	case 1:
		return copy(buffer, "first"), nil
	case 2:
		return copy(buffer, "last"), io.EOF
	default:
		return 0, io.EOF
	}
}

func TestWriteOutputRejectsPartialWrite(t *testing.T) {
	err := terminal.WriteOutput(partialWriter{}, []byte("output"))

	assert.ErrorIs(t, err, io.ErrShortWrite)
}

type partialWriter struct{}

func (partialWriter) Write(data []byte) (int, error) {
	return len(data) - 1, nil
}

func TestWriteOutputPreservesWriterError(t *testing.T) {
	wantErr := errors.New("output failed")

	err := terminal.WriteOutput(errorWriter{err: wantErr}, []byte("output"))

	assert.ErrorIs(t, err, wantErr)
}

type errorWriter struct {
	err error
}

func (writer errorWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

func TestReadInputTransfersOwnedChunksAndPreservesFinalBytes(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, reader.Close())
		assert.NoError(t, writer.Close())
	})

	_, err = writer.Write([]byte{1})
	require.NoError(t, err)

	var chunks [][]byte

	err = terminal.ReadInput(t.Context(), &finalBytesInput{file: reader}, 16, func(data []byte) error {
		chunks = append(chunks, data)

		return nil
	})

	require.ErrorIs(t, err, io.EOF)
	assert.Equal(t, [][]byte{[]byte("first"), []byte("last")}, chunks)
}

func TestReadInputCancellationDoesNotLeaveWorkers(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, reader.Close())
		assert.NoError(t, writer.Close())
	})

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() {
		done <- terminal.ReadInput(ctx, reader, 16, func([]byte) error { return nil })
	}()

	cancel()

	select {
	case err = <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("idle terminal input did not stop after cancellation")
	}
}

func TestReadInputRejectsInvalidDescriptor(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	t.Cleanup(func() { assert.NoError(t, writer.Close()) })

	err = terminal.ReadInput(t.Context(), reader, 16, func([]byte) error { return nil })

	require.Error(t, err)
	assert.NotErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, context.DeadlineExceeded)
}
