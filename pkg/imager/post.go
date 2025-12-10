// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/reporter"
)

//nolint:gocyclo
func (i *Imager) postProcessTar(ctx context.Context, filename string, report *reporter.Reporter) (string, error) {
	report.Report(reporter.Update{Message: "processing .tar.gz", Status: reporter.StatusRunning})

	dir := filepath.Dir(filename)
	src := "disk.raw"

	if err := os.Rename(filename, filepath.Join(dir, src)); err != nil {
		return "", err
	}

	outPath := filename + ".tar.gz"

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return "", err
	}

	timestamp, ok, err := utils.SourceDateEpoch()
	if err != nil {
		return "", fmt.Errorf("failed to get SOURCE_DATE_EPOCH: %w", err)
	}

	if !ok {
		timestamp = time.Now().Unix()
	}

	cmd1 := exec.CommandContext(
		ctx,
		"tar",
		"-cvf",
		"-",
		"-C",
		dir,
		"--sparse",
		"--sort=name",
		"--owner=0",
		"--group=0",
		"--numeric-owner",
		"--mtime=@"+strconv.FormatInt(timestamp, 10),
		src,
	)

	cmd1.Stdout = pipeW
	cmd1.Stderr = os.Stderr

	if err := cmd1.Start(); err != nil {
		return "", err
	}

	if err = pipeW.Close(); err != nil {
		return "", err
	}

	destination, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}

	defer destination.Close() //nolint:errcheck

	cmd2 := exec.CommandContext(ctx, "pigz", "-6", "-f", "--no-time", "-")
	cmd2.Stdin = pipeR
	cmd2.Stdout = destination
	cmd2.Stderr = os.Stderr

	if err := cmd2.Start(); err != nil {
		return "", err
	}

	if err = pipeR.Close(); err != nil {
		return "", err
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- cmd1.Wait()
	}()

	go func() {
		errCh <- cmd2.Wait()
	}()

	for range 2 {
		if err = <-errCh; err != nil {
			return "", err
		}
	}

	if err := destination.Sync(); err != nil {
		return "", err
	}

	if err := os.Remove(filepath.Join(dir, src)); err != nil {
		return "", err
	}

	report.Report(reporter.Update{Message: fmt.Sprintf("archive is ready: %s", outPath), Status: reporter.StatusSucceeded})

	return outPath, nil
}

func (i *Imager) postProcessGz(filename string, report *reporter.Reporter) (string, error) {
	report.Report(reporter.Update{Message: "compressing .gz", Status: reporter.StatusRunning})

	if _, err := cmd.Run("pigz", "-6", "--no-time", "-f", filename); err != nil {
		return "", err
	}

	report.Report(reporter.Update{Message: fmt.Sprintf("compression done: %s.gz", filename), Status: reporter.StatusSucceeded})

	return filename + ".gz", nil
}

func (i *Imager) postProcessXz(filename string, report *reporter.Reporter) (string, error) {
	report.Report(reporter.Update{Message: "compressing .xz", Status: reporter.StatusRunning})

	if _, err := cmd.Run("xz", "-0", "-f", "-T", "0", filename); err != nil {
		return "", err
	}

	report.Report(reporter.Update{Message: fmt.Sprintf("compression done: %s.xz", filename), Status: reporter.StatusSucceeded})

	return filename + ".xz", nil
}

func (i *Imager) postProcessZstd(filename string, report *reporter.Reporter) (string, error) {
	report.Report(reporter.Update{Message: "compressing .zst", Status: reporter.StatusRunning})

	out := filename + ".zst"

	if _, err := cmd.Run("zstd", "-T0", "--rm", "-18", "--quiet", "--force", "-o", out, filename); err != nil {
		return "", err
	}

	report.Report(reporter.Update{Message: fmt.Sprintf("compression done: %s", out), Status: reporter.StatusSucceeded})

	return filename + ".zst", nil
}
