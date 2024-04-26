// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/pkg/reporter"
)

func (i *Imager) postProcessTar(filename string, report *reporter.Reporter) (string, error) {
	report.Report(reporter.Update{Message: "processing .tar.gz", Status: reporter.StatusRunning})

	dir := filepath.Dir(filename)
	src := "disk.raw"

	if err := os.Rename(filename, filepath.Join(dir, src)); err != nil {
		return "", err
	}

	outPath := filename + ".tar.gz"

	if _, err := cmd.Run("tar", "-cvf", outPath, "-C", dir, "--sparse", "--use-compress-program=pigz -6", src); err != nil {
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

	if _, err := cmd.Run("pigz", "-6", "-f", filename); err != nil {
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
