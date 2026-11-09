// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemuimg_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/qemuimg"
)

func TestIndependentConversion(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if override := os.Getenv("TALOS_TEST_QEMU_IMG"); override != "" {
		binary, err = override, nil
	}

	if err != nil {
		t.Skip("qemu-img is required for real image qualification")
	}

	client := qemuimg.New(binary)

	for _, format := range []string{"raw", "qcow2"} {
		t.Run(format, func(t *testing.T) {
			source, err := os.CreateTemp(t.TempDir(), "source")
			require.NoError(t, err)

			defer source.Close() //nolint:errcheck

			require.NoError(t, source.Truncate(1<<20))
			_, err = source.WriteAt([]byte("guest contents"), 512)
			require.NoError(t, err)
			destination, err := os.Create(filepath.Join(t.TempDir(), "destination"))
			require.NoError(t, err)

			defer destination.Close() //nolint:errcheck

			require.NoError(t, client.Convert(t.Context(), qemuimg.ConvertSpec{
				Source:      source,
				Destination: destination,
				Format:      format,
				Capacity:    2 << 20,
			}))
			info, err := client.Info(t.Context(), destination)
			require.NoError(t, err)
			require.Equal(t, format, info.Format)
			require.EqualValues(t, 2<<20, info.VirtualSize)
			require.Empty(t, info.BackingFile)
			require.Empty(t, info.FormatSpecific.Data.DataFile)
		})
	}
}

func TestRawDestinationWithQCOWMagic(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	dir := t.TempDir()
	inner := filepath.Join(dir, "inner.qcow2")
	output, err := exec.CommandContext(t.Context(), binary, "create", "-f", "qcow2", inner, "1M").CombinedOutput()
	require.NoError(t, err, "%s", output)

	guest := filepath.Join(dir, "guest.raw")
	require.NoError(t, os.WriteFile(guest, make([]byte, 2<<20), 0o600))

	header, err := os.ReadFile(inner)
	require.NoError(t, err)
	file, err := os.OpenFile(guest, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = file.WriteAt(header, 0)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	sourcePath := filepath.Join(dir, "source.qcow2")
	output, err = exec.CommandContext(t.Context(), binary, "convert", "-f", "raw", "-O", "qcow2", guest, sourcePath).CombinedOutput()
	require.NoError(t, err, "%s", output)
	source, err := os.Open(sourcePath)
	require.NoError(t, err)

	defer source.Close() //nolint:errcheck

	destination, err := os.Create(filepath.Join(dir, "destination.raw"))
	require.NoError(t, err)

	defer destination.Close() //nolint:errcheck

	client := qemuimg.New(binary)
	require.NoError(t, client.Convert(t.Context(), qemuimg.ConvertSpec{
		Source:      source,
		Destination: destination,
		Format:      "raw",
		Capacity:    3 << 20,
	}))
	infoOutput, err := exec.CommandContext(t.Context(), binary, "info", "--output=json", "-f", "raw", destination.Name()).CombinedOutput()
	require.NoError(t, err, "%s", infoOutput)

	var info qemuimg.Info
	require.NoError(t, json.Unmarshal(infoOutput, &info))
	require.EqualValues(t, 3<<20, info.VirtualSize)
}

func TestRejectDependenciesBeforeStartingTool(t *testing.T) {
	for _, kind := range []string{"backing", "data", "version", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "image")
			require.NoError(t, err)

			defer file.Close() //nolint:errcheck

			header := make([]byte, 104)
			copy(header, "QFI\xfb")
			binary.BigEndian.PutUint32(header[4:8], 3)

			switch kind {
			case "backing":
				binary.BigEndian.PutUint64(header[8:16], 104)
			case "data":
				binary.BigEndian.PutUint64(header[72:80], 1<<2)
			case "version":
				binary.BigEndian.PutUint32(header[4:8], 100)
			case "truncated":
				header = header[:4]
			}

			_, err = file.Write(header)
			require.NoError(t, err)
			_, err = qemuimg.New("/must-not-execute").Info(t.Context(), file)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "must-not-execute")
		})
	}
}

func TestRejectKnownNonRawSources(t *testing.T) {
	vdi := make([]byte, 104)
	binary.LittleEndian.PutUint32(vdi[64:68], 0xbeda107f)

	for name, header := range map[string][]byte{
		"zstd": []byte("\x28\xb5\x2f\xfd rest"),
		"xz":   []byte("\xfd7zXZ\x00 rest"),
		"gzip": []byte("\x1f\x8b\x08 rest"),
		"vmdk": []byte("KDMV rest"),
		"vhdx": []byte("vhdxfile rest"),
		"vdi":  vdi,
	} {
		t.Run(name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "image")
			require.NoError(t, err)

			defer file.Close() //nolint:errcheck

			_, err = file.Write(header)
			require.NoError(t, err)

			_, err = qemuimg.Inspect(file)
			require.ErrorContains(t, err, "only uncompressed raw or qcow2")
			require.True(t, qemuimg.IsTerminal(err))
		})
	}
}

func TestSelfContainedExtendedQCOW2(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	path := filepath.Join(t.TempDir(), "extended.qcow2")
	output, err := exec.CommandContext(t.Context(), binary, "create", "-f", "qcow2", "-o", "extended_l2=on", path, "1M").CombinedOutput()
	require.NoError(t, err, "%s", output)
	file, err := os.Open(path)
	require.NoError(t, err)

	defer file.Close() //nolint:errcheck

	info, err := qemuimg.New(binary).Info(t.Context(), file)
	require.NoError(t, err)
	require.EqualValues(t, 1<<20, info.VirtualSize)
}

func TestInitialCapacityUsesVirtualSize(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	path := filepath.Join(t.TempDir(), "source.qcow2")
	output, err := exec.CommandContext(t.Context(), binary, "create", "-f", "qcow2", path, "8M").CombinedOutput()
	require.NoError(t, err, "%s", output)
	source, err := os.Open(path)
	require.NoError(t, err)

	defer source.Close() //nolint:errcheck

	stat, err := source.Stat()
	require.NoError(t, err)
	require.Less(t, stat.Size(), int64(4<<20))
	destination, err := os.CreateTemp(t.TempDir(), "destination")
	require.NoError(t, err)

	defer destination.Close() //nolint:errcheck

	client := qemuimg.New(binary)
	err = client.Convert(t.Context(), qemuimg.ConvertSpec{
		Source:      source,
		Destination: destination,
		Format:      "qcow2",
		Capacity:    4 << 20,
	})
	require.ErrorContains(t, err, "below source virtual size")
	require.NoError(t, client.Convert(t.Context(), qemuimg.ConvertSpec{
		Source:      source,
		Destination: destination,
		Format:      "qcow2",
		Capacity:    8 << 20,
	}))
	info, err := client.Info(t.Context(), destination)
	require.NoError(t, err)
	require.EqualValues(t, 8<<20, info.VirtualSize)
}

func TestMetadataAndCommandFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "nested data file",
			body: `printf '{"format":"raw","virtual-size":8,"format-specific":{"type":"qcow2","data":{"data-file":"private.raw"}}}'`,
			want: "external dependencies",
		},
		{
			name: "truncated",
			body: "printf '{'",
			want: "decode",
		},
		{
			name: "missing size",
			body: `printf '{"format":"raw"}'`,
			want: "virtual size",
		},
		{
			name: "wrong format",
			body: `printf '{"format":"vmdk","virtual-size":8}'`,
			want: "unsupported",
		},
		{
			name: "bounded",
			body: "head -c 70000 /dev/zero",
			want: "output limit",
		},
		{
			name: "exit",
			body: "echo diagnostic >&2; exit 42",
			want: "exit code 42",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			tool := filepath.Join(t.TempDir(), "tool")
			require.NoError(t, os.WriteFile(tool, []byte("#!/bin/sh\n"+test.body+"\n"), 0o700))
			file, err := os.CreateTemp(t.TempDir(), "source")
			require.NoError(t, err)

			defer file.Close() //nolint:errcheck

			_, err = qemuimg.New(tool).Info(t.Context(), file)
			require.ErrorContains(t, err, test.want)

			if test.name == "exit" {
				var failure *qemuimg.ExecError
				require.ErrorAs(t, err, &failure)
				require.Equal(t, 42, failure.ExitCode)
				require.Contains(t, string(failure.Stderr), "diagnostic")
			}
		})
	}
}

func TestCancellationWaitsForChildExit(t *testing.T) {
	directory := t.TempDir()
	pidfile := filepath.Join(directory, "pid")
	tool := filepath.Join(directory, "tool")
	require.NoError(t, os.WriteFile(tool, []byte("#!/bin/sh\nread hostpid rest < /proc/self/stat\necho \"$hostpid\" > '"+pidfile+"'\nexec sleep 300\n"), 0o700))
	file, err := os.CreateTemp(t.TempDir(), "source")
	require.NoError(t, err)

	defer file.Close() //nolint:errcheck

	client := qemuimg.New(tool)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() {
		_, err := client.Info(ctx, file)
		done <- err
	}()

	defer func() {
		cancel()
		<-done
	}()

	require.Eventually(t, func() bool {
		_, err := os.Stat(pidfile)

		return err == nil
	}, 3*time.Second, time.Millisecond)

	pid, err := os.ReadFile(pidfile)
	require.NoError(t, err)
	cancel()

	err = <-done
	done <- err

	require.ErrorIs(t, err, context.Canceled)

	var number int

	_, err = fmt.Sscan(string(pid), &number)
	require.NoError(t, err)
	_, err = os.Stat(fmt.Sprintf("/proc/%d", number))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestOpenFilesSurviveChildGC(t *testing.T) {
	for _, operation := range []string{"info", "convert", "resize"} {
		t.Run(operation, func(t *testing.T) {
			directory := t.TempDir()
			ready := filepath.Join(directory, "ready")
			release := filepath.Join(directory, "release")
			tool := filepath.Join(directory, "tool")
			// Delay opening the parent's descriptors until GC/finalizers have
			// completed. No caller reference or deferred Close protects them.
			script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = '%s' ]; then
    echo ready > '%s'
    while ! test -f '%s'; do sleep 0.01; done
fi
previous=
last=
for argument do previous=$last; last=$argument; done
case "$1" in
    info) cat "$last" > /dev/null || exit 23
        printf '{"format":"raw","virtual-size":4096}' ;;
    convert) cat "$previous" > "$last" || exit 23 ;;
    resize) cat "$previous" > /dev/null || exit 23 ;;
esac
`, operation, ready, release)
			require.NoError(t, os.WriteFile(tool, []byte(script), 0o700))

			source := filepath.Join(directory, "source")
			destination := filepath.Join(directory, "destination")

			require.NoError(t, os.WriteFile(source, make([]byte, 4096), 0o600))
			require.NoError(t, os.WriteFile(destination, make([]byte, 4096), 0o600))

			client := qemuimg.New(tool)
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)

			go func() {
				file, err := lifetimeFile(source)
				if err != nil {
					done <- err

					return
				}

				switch operation {
				case "info":
					_, err = client.Info(ctx, file)
				case "resize":
					err = client.Resize(ctx, file, "raw", 8192)
				case "convert":
					var output *os.File

					output, err = lifetimeFile(destination)
					if err == nil {
						err = client.Convert(ctx, qemuimg.ConvertSpec{Source: file, Destination: output, Format: "raw", Capacity: 4096})
					}
				}

				done <- err
			}()

			defer func() { cancel(); <-done }()

			require.Eventually(t, func() bool {
				_, err := os.Stat(ready)

				return err == nil
			}, 3*time.Second, time.Millisecond)
			// Finalizers run serially. Multiple completed GC/finalizer barriers
			// drain candidates without guessing how long the scheduler needs.
			for range 3 {
				finalized := make(chan struct{})
				finalizerBarrier(finalized)
				runtime.GC()

				select {
				case <-finalized:
				case <-time.After(3 * time.Second):
					t.Fatal("GC finalizer barrier did not complete")
				}
			}

			require.NoError(t, os.WriteFile(release, nil, 0o600))

			err := <-done
			done <- err

			require.NoError(t, err, "child must still be able to open every supplied descriptor after GC")
		})
	}
}

func lifetimeFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	runtime.SetFinalizer(file, func(file *os.File) { file.Close() }) //nolint:errcheck

	return file, nil
}

// A non-tiny allocation avoids Go's batched tiny-allocation finalizer behavior.
func finalizerBarrier(finalized chan struct{}) {
	sentinel := new([1024]byte)
	runtime.SetFinalizer(sentinel, func(*[1024]byte) { close(finalized) })
}

func TestQCOW2CapacityRoundsUpToSector(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	tool := filepath.Join(dir, "tool")
	script := `#!/bin/sh
echo "$@" >> '` + log + `'
format=raw
while [ $# -gt 0 ]; do [ "$1" = -f ] && format=$2; shift; done
printf '{"format":"%s","virtual-size":512}' "$format"
`
	require.NoError(t, os.WriteFile(tool, []byte(script), 0o700))

	source, err := os.CreateTemp(dir, "source")
	require.NoError(t, err)

	defer source.Close() //nolint:errcheck

	require.NoError(t, source.Truncate(512))

	destination, err := os.CreateTemp(dir, "destination")
	require.NoError(t, err)

	defer destination.Close() //nolint:errcheck

	var result qemuimg.Info

	require.NoError(t, qemuimg.New(tool).Convert(t.Context(), qemuimg.ConvertSpec{
		Source: source, Destination: destination, Format: "qcow2", Capacity: 1000001, Result: &result,
	}))
	require.EqualValues(t, 1000448, result.VirtualSize)

	calls, err := os.ReadFile(log)
	require.NoError(t, err)
	require.Contains(t, string(calls), " 1000448\n")
}

func TestExecutionDiagnosticBoundedAndSanitized(t *testing.T) {
	err := &qemuimg.ExecError{ExitCode: 1, Stderr: []byte("No space left on device\n\x1b[31m\x00\r" + strings.Repeat("x", 70000))}
	message := err.Error()
	require.Contains(t, message, "No space left on device")
	require.LessOrEqual(t, len(message), 1200)

	for _, r := range message {
		require.False(t, unicode.IsControl(r), "status diagnostic contains control %q", r)
	}
}

func TestTerminalClassificationUsesTypes(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "source")
	require.NoError(t, err)

	defer file.Close() //nolint:errcheck

	require.NoError(t, file.Truncate(512))
	_, err = qemuimg.New("/missing/qemu-img").Info(t.Context(), file)
	require.True(t, qemuimg.IsTerminal(err))
	tool := filepath.Join(t.TempDir(), "tool")
	require.NoError(t, os.WriteFile(tool, []byte("#!/bin/sh\necho 'No space left on device' >&2\nexit 1\n"), 0o700))
	_, err = qemuimg.New(tool).Info(t.Context(), file)
	require.ErrorContains(t, err, "No space left on device")
	require.False(t, qemuimg.IsTerminal(err))
	require.False(t, qemuimg.IsTerminal(&qemuimg.ExecError{ExitCode: 1, Stderr: []byte("unsupported image")}))
	require.False(t, qemuimg.IsTerminal(context.Canceled))
}

func TestRealNestedDataFileMetadata(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	dir := t.TempDir()
	image := filepath.Join(dir, "external.qcow2")
	output, err := exec.CommandContext(t.Context(), binary, "create", "-f", "qcow2", "-o", "data_file="+filepath.Join(dir, "data.raw")+",data_file_raw=on", image, "1M").CombinedOutput()
	require.NoError(t, err, "%s", output)
	output, err = exec.CommandContext(t.Context(), binary, "info", "--output=json", image).CombinedOutput()
	require.NoError(t, err, "%s", output)

	var info qemuimg.Info
	require.NoError(t, json.Unmarshal(output, &info))
	require.Equal(t, filepath.Join(dir, "data.raw"), info.FormatSpecific.Data.DataFile)
	t.Logf("real nested metadata: %s", output)

	source, err := os.Open(image)
	require.NoError(t, err)

	defer source.Close() //nolint:errcheck

	_, err = qemuimg.New(binary).Info(t.Context(), source)
	require.True(t, qemuimg.IsTerminal(err), "%v", err)
	require.ErrorContains(t, err, "external data files")
}

func TestParserPinsRenamedSource(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "source")
	require.NoError(t, os.WriteFile(path, make([]byte, 512), 0o600))
	source, err := os.Open(path)
	require.NoError(t, err)

	defer source.Close() //nolint:errcheck

	entry := filepath.Join(dir, "rename-before-qemu")
	script := fmt.Sprintf("#!/bin/sh\nmv '%s' '%s.held'\nprintf 'QFI' > '%s'\nexec '%s' \"$@\"\n", path, path, path, binary)
	require.NoError(t, os.WriteFile(entry, []byte(script), 0o700))
	info, err := qemuimg.New(entry).Info(t.Context(), source)
	require.NoError(t, err)
	require.EqualValues(t, 512, info.VirtualSize)
	require.Equal(t, "raw", info.Format)

	replacement, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "QFI", string(replacement))
}
