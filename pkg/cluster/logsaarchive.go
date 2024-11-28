// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/siderolabs/talos/pkg/imager/filemap"
)

// SaveClusterLogsArchive saves all logs from the cluster state directory to a gzip archive.
func SaveClusterLogsArchive(statePath, archivePath string) {
	if err := saveClusterLogsArchive(statePath, archivePath); err != nil {
		fmt.Fprintf(os.Stderr, "error saving cluster logs archive: %v\n", err)
	}
}

func saveClusterLogsArchive(statePath, archivePath string) error {
	var logFileMap []filemap.File

	if err := filepath.WalkDir(statePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".log") {
			return nil
		}

		rel, err := filepath.Rel(statePath, path)
		if err != nil {
			return err
		}

		if d.IsDir() && rel == "." {
			return nil
		}

		statInfo, err := d.Info()
		if err != nil {
			return err
		}

		logFileMap = append(logFileMap, filemap.File{
			ImagePath:  rel,
			SourcePath: path,
			ImageMode:  int64(statInfo.Mode().Perm()),
		})

		return nil
	}); err != nil {
		return fmt.Errorf("error building filemap: %w", err)
	}

	logFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("error creating log archive: %w", err)
	}

	defer logFile.Close() //nolint:errcheck

	gzipWriter := gzip.NewWriter(logFile)
	defer gzipWriter.Close() //nolint:errcheck

	r := filemap.Build(logFileMap)

	if _, err := io.Copy(gzipWriter, r); err != nil {
		return fmt.Errorf("error writing log archive: %w", err)
	}

	return nil
}
