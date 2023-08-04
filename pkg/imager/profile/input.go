// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/archiver"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Input describes inputs for image generation.
type Input struct {
	// Kernel is a vmlinuz file.
	Kernel FileAsset `yaml:"kernel"`
	// Initramfs is a initramfs file (without system extensions).
	Initramfs FileAsset `yaml:"initramfs"`
	// SDStub is a sd-stub file (only for SecureBoot).
	SDStub FileAsset `yaml:"sdStub,omitempty"`
	// SDBoot is a sd-boot file (only for SecureBoot).
	SDBoot FileAsset `yaml:"sdBoot,omitempty"`
	// Base installer image to mutate.
	BaseInstaller ContainerAsset `yaml:"baseInstaller,omitempty"`
	// SecureBoot is a section with secureboot keys, only for SecureBoot enabled builds.
	SecureBoot *SecureBootAssets `yaml:"secureboot,omitempty"`
	// SystemExtensions is a list of system extensions to install.
	SystemExtensions []ContainerAsset `yaml:"systemExtensions,omitempty"`
}

// FileAsset describes a file asset.
type FileAsset struct {
	// Path to the file.
	Path string `yaml:"path"`
}

// ContainerAsset describes a container asset.
type ContainerAsset struct {
	// ImageRef is a reference to the container image.
	ImageRef string `yaml:"imageRef"`
}

// SecureBootAssets describes secureboot assets.
type SecureBootAssets struct {
	// SecureBoot signing key & cert.
	SigningKeyPath  string `yaml:"signingKeyPath"`
	SigningCertPath string `yaml:"signingCertPath"`
	// PCR signing key & public key.
	PCRSigningKeyPath string `yaml:"pcrSigningKeyPath"`
	PCRPublicKeyPath  string `yaml:"pcrPublicKeyPath"`
	// Optional, auto-enrollment paths.
	PlatformKeyPath    string `yaml:"platformKeyPath,omitempty"`
	KeyExchangeKeyPath string `yaml:"keyExchangeKeyPath,omitempty"`
	SignatureKeyPath   string `yaml:"signatureKeyPath,omitempty"`
}

const defaultSecureBootPrefix = "/secureboot"

// FillDefaults fills default values for the input.
//
//nolint:gocyclo
func (i *Input) FillDefaults(arch, version string, secureboot bool) {
	var (
		zeroFileAsset      FileAsset
		zeroContainerAsset ContainerAsset
	)

	if i.Kernel == zeroFileAsset {
		i.Kernel.Path = fmt.Sprintf(constants.KernelAssetPath, arch)
	}

	if i.Initramfs == zeroFileAsset {
		i.Initramfs.Path = fmt.Sprintf(constants.InitramfsAssetPath, arch)
	}

	if i.BaseInstaller == zeroContainerAsset {
		i.BaseInstaller.ImageRef = fmt.Sprintf("%s:%s", images.DefaultInstallerImageRepository, version)
	}

	if secureboot {
		if i.SDStub == zeroFileAsset {
			i.SDStub.Path = fmt.Sprintf(constants.SDStubAssetPath, arch)
		}

		if i.SDBoot == zeroFileAsset {
			i.SDBoot.Path = fmt.Sprintf(constants.SDBootAssetPath, arch)
		}

		if i.SecureBoot == nil {
			i.SecureBoot = &SecureBootAssets{}
		}

		if i.SecureBoot.SigningKeyPath == "" {
			i.SecureBoot.SigningKeyPath = filepath.Join(defaultSecureBootPrefix, constants.SecureBootSigningKeyAsset)
		}

		if i.SecureBoot.SigningCertPath == "" {
			i.SecureBoot.SigningCertPath = filepath.Join(defaultSecureBootPrefix, constants.SecureBootSigningCertAsset)
		}

		if i.SecureBoot.PCRSigningKeyPath == "" {
			i.SecureBoot.PCRSigningKeyPath = filepath.Join(defaultSecureBootPrefix, constants.PCRSigningKeyAsset)
		}

		if i.SecureBoot.PCRPublicKeyPath == "" {
			i.SecureBoot.PCRPublicKeyPath = filepath.Join(defaultSecureBootPrefix, constants.PCRSigningPublicKeyAsset)
		}

		if i.SecureBoot.PlatformKeyPath == "" {
			if platformKeyPath := filepath.Join(defaultSecureBootPrefix, constants.PlatformKeyAsset); fileExists(platformKeyPath) {
				i.SecureBoot.PlatformKeyPath = platformKeyPath
			}
		}

		if i.SecureBoot.KeyExchangeKeyPath == "" {
			if keyExchangeKeyPath := filepath.Join(defaultSecureBootPrefix, constants.KeyExchangeKeyAsset); fileExists(keyExchangeKeyPath) {
				i.SecureBoot.KeyExchangeKeyPath = keyExchangeKeyPath
			}
		}

		if i.SecureBoot.SignatureKeyPath == "" {
			if signatureKeyPath := filepath.Join(defaultSecureBootPrefix, constants.SignatureKeyAsset); fileExists(signatureKeyPath) {
				i.SecureBoot.SignatureKeyPath = signatureKeyPath
			}
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// Pull the container asset to the path.
func (c *ContainerAsset) Pull(ctx context.Context, arch string, printf func(string, ...any)) (v1.Image, error) {
	printf("pulling %s...", c.ImageRef)

	img, err := crane.Pull(c.ImageRef, crane.WithPlatform(&v1.Platform{
		Architecture: arch,
		OS:           "linux",
	}), crane.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error pulling image %s: %w", c.ImageRef, err)
	}

	return img, nil
}

// Extract the container asset to the path.
func (c *ContainerAsset) Extract(ctx context.Context, destination, arch string, printf func(string, ...any)) error {
	img, err := c.Pull(ctx, arch, printf)
	if err != nil {
		return err
	}

	r, w := io.Pipe()

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		defer w.Close() //nolint:errcheck

		return crane.Export(img, w)
	})

	eg.Go(func() error {
		return archiver.Untar(ctx, r, destination)
	})

	return eg.Wait()
}
