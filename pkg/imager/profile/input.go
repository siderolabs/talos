// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/siderolabs/gen/value"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/internal/pkg/secureboot/measure"
	"github.com/siderolabs/talos/internal/pkg/secureboot/pesign"
	"github.com/siderolabs/talos/pkg/archiver"
	"github.com/siderolabs/talos/pkg/imager/profile/internal/signer/aws"
	"github.com/siderolabs/talos/pkg/imager/profile/internal/signer/azure"
	"github.com/siderolabs/talos/pkg/imager/profile/internal/signer/file"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	arm64 = "arm64"
	amd64 = "amd64"
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
	// DTB is a path to the device tree blobs (arm64 only).
	DTB FileAsset `yaml:"dtb,omitempty"`
	// UBoot is a path to the u-boot binary (arm64 only).
	UBoot FileAsset `yaml:"uBoot,omitempty"`
	// RPiFirmware is a path to the Raspberry Pi firmware (arm64 only).
	RPiFirmware FileAsset `yaml:"rpiFirmware,omitempty"`
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
	// ForceInsecure forces insecure registry communication.
	ForceInsecure bool `yaml:"forceInsecure,omitempty"`
	// TarballPath is a path to the .tar format container image contents.
	//
	// If TarballPath is set, ImageRef is ignored.
	TarballPath string `yaml:"tarballPath,omitempty"`
	// OCIPath is a path to the OCI format container image contents.
	//
	// If OCIPath is set, ImageRef is ignored.
	OCIPath string `yaml:"ociPath,omitempty"`

	// AuthConfig is a authentication config to access private registry.
	*authn.AuthConfig
}

// SecureBootAssets describes secureboot assets.
type SecureBootAssets struct {
	// SecureBoot signing key & cert.
	SecureBootSigner SigningKeyAndCertificate `yaml:"secureBootSigner"`
	// PCR signing key.
	PCRSigner SigningKey `yaml:"pcrSigner"`
	// Optional, auto-enrollment paths.
	PlatformKeyPath    string `yaml:"platformKeyPath,omitempty"`
	KeyExchangeKeyPath string `yaml:"keyExchangeKeyPath,omitempty"`
	SignatureKeyPath   string `yaml:"signatureKeyPath,omitempty"`
}

// SigningKeyAndCertificate describes a signing key & certificate.
type SigningKeyAndCertificate struct {
	// File-based.
	//
	// Static key and certificate paths.
	KeyPath  string `yaml:"keyPath,omitempty"`
	CertPath string `yaml:"certPath,omitempty"`
	// Azure.
	//
	// Azure Vault URL and certificate ID, key will be found from the certificate.
	AzureVaultURL      string `yaml:"azureVaultURL,omitempty"`
	AzureCertificateID string `yaml:"azureCertificateID,omitempty"`
	// AWS.
	//
	// AWS KMS Key ID and region.
	// AWS doesn't have a good way to store a certificate, so it's expected to be a file.
	AwsKMSKeyID string `yaml:"awsKMSKeyID,omitempty"`
	AwsRegion   string `yaml:"awsRegion,omitempty"`
	AwsCertPath string `yaml:"awsCertPath,omitempty"`
}

// SigningKey describes a signing key.
type SigningKey struct {
	// File-based.
	//
	// Static key path.
	KeyPath string `yaml:"keyPath,omitempty"`
	// Azure.
	//
	// Azure Vault URL and key ID.
	// AzureKeyVersion might be left empty to use the latest key version.
	AzureVaultURL   string `yaml:"azureVaultURL,omitempty"`
	AzureKeyID      string `yaml:"azureKeyID,omitempty"`
	AzureKeyVersion string `yaml:"azureKeyVersion,omitempty"`
	// AWS.
	//
	// AWS KMS Key ID and region.
	AwsKMSKeyID string `yaml:"awsKMSKeyID,omitempty"`
	AwsRegion   string `yaml:"awsRegion,omitempty"`
}

// GetSigner returns the signer.
func (key SigningKey) GetSigner(ctx context.Context) (measure.RSAKey, error) {
	switch {
	case key.KeyPath != "":
		return file.NewPCRSigner(key.KeyPath)
	case key.AzureVaultURL != "" && key.AzureKeyID != "":
		return azure.NewPCRSigner(ctx, key.AzureVaultURL, key.AzureKeyID, key.AzureKeyVersion)
	case key.AwsKMSKeyID != "":
		return aws.NewPCRSigner(ctx, key.AwsKMSKeyID, key.AwsRegion)
	default:
		return nil, errors.New("unsupported PCR signer")
	}
}

// GetSigner returns the signer.
func (keyAndCert SigningKeyAndCertificate) GetSigner(ctx context.Context) (pesign.CertificateSigner, error) {
	switch {
	case keyAndCert.KeyPath != "" && keyAndCert.CertPath != "":
		return file.NewSecureBootSigner(keyAndCert.CertPath, keyAndCert.KeyPath)
	case keyAndCert.AzureVaultURL != "" && keyAndCert.AzureCertificateID != "":
		return azure.NewSecureBootSigner(ctx, keyAndCert.AzureVaultURL, keyAndCert.AzureCertificateID, keyAndCert.AzureCertificateID)
	case keyAndCert.AwsKMSKeyID != "" && keyAndCert.AwsCertPath != "":
		return aws.NewSecureBootSigner(ctx, keyAndCert.AwsKMSKeyID, keyAndCert.AwsRegion, keyAndCert.AwsCertPath)
	default:
		return nil, errors.New("unsupported PCR signer")
	}
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

	if arch == arm64 {
		if i.DTB == zeroFileAsset {
			i.DTB.Path = fmt.Sprintf(constants.DTBAssetPath, arch)
		}

		if i.UBoot == zeroFileAsset {
			i.UBoot.Path = fmt.Sprintf(constants.UBootAssetPath, arch)
		}

		if i.RPiFirmware == zeroFileAsset {
			i.RPiFirmware.Path = fmt.Sprintf(constants.RPiFirmwareAssetPath, arch)
		}
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

		if value.IsZero(i.SecureBoot.SecureBootSigner) {
			i.SecureBoot.SecureBootSigner.KeyPath = filepath.Join(defaultSecureBootPrefix, constants.SecureBootSigningKeyAsset)
			i.SecureBoot.SecureBootSigner.CertPath = filepath.Join(defaultSecureBootPrefix, constants.SecureBootSigningCertAsset)
		}

		if value.IsZero(i.SecureBoot.PCRSigner) {
			i.SecureBoot.PCRSigner.KeyPath = filepath.Join(defaultSecureBootPrefix, constants.PCRSigningKeyAsset)
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
	if c.TarballPath != "" {
		return nil, errors.New("pulling tarball container image is not supported")
	}

	if c.OCIPath != "" {
		printf("using OCI image from %s...", c.OCIPath)

		return c.pullFromOCI(arch)
	}

	printf("pulling %s...", c.ImageRef)

	opts := []crane.Option{
		crane.WithPlatform(&v1.Platform{
			Architecture: arch,
			OS:           "linux",
		}),
		crane.WithContext(ctx),
	}

	if auth, _ := c.Authorization(); auth != nil { //nolint:errcheck
		opts = append(opts, crane.WithAuth(c))
	}

	if c.ForceInsecure {
		opts = append(opts, crane.Insecure)
	}

	img, err := crane.Pull(c.ImageRef, opts...)
	if err != nil {
		return nil, fmt.Errorf("error pulling image %s: %w", c.ImageRef, err)
	}

	return img, nil
}

// Authorization fullfils container registry Authorization interface.
func (c *ContainerAsset) Authorization() (*authn.AuthConfig, error) {
	return c.AuthConfig, nil
}

func (c *ContainerAsset) pullFromOCI(arch string) (v1.Image, error) {
	ociLayout, err := layout.FromPath(c.OCIPath)
	if err != nil {
		return nil, fmt.Errorf("error opening OCI layout: %w", err)
	}

	ociIndex, err := ociLayout.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("error opening OCI index: %w", err)
	}

	ociManifest, err := ociIndex.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("error opening OCI manifest: %w", err)
	}

	for _, manifest := range ociManifest.Manifests {
		if manifest.Platform == nil {
			continue
		}

		if manifest.Platform.OS == "linux" && manifest.Platform.Architecture == arch {
			img, err := ociLayout.Image(manifest.Digest)
			if err != nil {
				return nil, fmt.Errorf("error opening OCI image: %w", err)
			}

			return img, nil
		}
	}

	return nil, fmt.Errorf("no OCI image found for %s", arch)
}

// Extract the container asset to the path.
func (c *ContainerAsset) Extract(ctx context.Context, destination, arch string, printf func(string, ...any)) error {
	if c.TarballPath != "" {
		in, err := os.Open(c.TarballPath)
		if err != nil {
			return err
		}

		defer in.Close() //nolint:errcheck

		printf("extracting %s...", c.TarballPath)

		return archiver.Untar(ctx, in, destination)
	}

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
