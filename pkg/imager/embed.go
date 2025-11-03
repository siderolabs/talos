// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
)

func (i *Imager) handleEmbeddedConfig() error {
	if len(i.prof.Customization.EmbeddedMachineConfiguration) == 0 {
		return nil
	}

	contents, err := BuildEmbeddedConfigExtension([]byte(i.prof.Customization.EmbeddedMachineConfiguration))
	if err != nil {
		return fmt.Errorf("failed to build embedded config extension: %w", err)
	}

	tmpPath := filepath.Join(i.tempDir, "embedded-config.tar")

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	if _, err := io.Copy(f, contents); err != nil {
		return fmt.Errorf("failed to write embedded config to temporary file: %w", err)
	}

	i.prof.Input.SystemExtensions = append(i.prof.Input.SystemExtensions,
		profile.ContainerAsset{
			TarballPath: tmpPath,
		},
	)

	return nil
}

// BuildEmbeddedConfigExtension builds a tarball containing the embedded machine configuration as a virtual extension.
func BuildEmbeddedConfigExtension(machineConfig []byte) (io.Reader, error) {
	sha256sum := sha256.Sum256(machineConfig)
	extensionVersion := hex.EncodeToString(sha256sum[:])

	manifest := extensions.Manifest{
		Version: "v1alpha1",
		Metadata: extensions.Metadata{
			Name:        "embedded-config",
			Version:     extensionVersion,
			Author:      "Imager",
			Description: "Virtual extension which embeds the machine configuration.",
			Compatibility: extensions.Compatibility{
				Talos: extensions.Constraint{
					Version: ">= 1.0.0",
				},
			},
		},
	}

	manifestBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	if err = tw.WriteHeader(&tar.Header{
		Name:     "manifest.yaml",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(manifestBytes)),
	}); err != nil {
		return nil, fmt.Errorf("failed to write manifest header: %w", err)
	}

	if _, err = tw.Write(manifestBytes); err != nil {
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	path := strings.Split(strings.TrimRight(constants.EmbeddedConfigDirectory, "/"), "/")

	for i := range path {
		dir := filepath.Join("rootfs", strings.Join(path[:i+1], "/"))

		if err = tw.WriteHeader(&tar.Header{
			Name:     dir,
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		}); err != nil {
			return nil, fmt.Errorf("failed to write rootfs header: %w", err)
		}
	}

	if err = tw.WriteHeader(&tar.Header{
		Name:     filepath.Join("rootfs", constants.EmbeddedConfigDirectory, constants.ConfigFilename),
		Typeflag: tar.TypeReg,
		Mode:     0o000,
		Size:     int64(len(machineConfig)),
		PAXRecords: map[string]string{
			"SCHILY.xattr.security.selinux": constants.StateSelinuxLabel,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to write embedded header: %w", err)
	}

	if _, err = tw.Write(machineConfig); err != nil {
		return nil, fmt.Errorf("failed to write embedded config: %w", err)
	}

	if err = tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	return &buf, nil
}
