// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uki

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"

	talosx509 "github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/internal/pkg/secureboot/measure"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/splash"
)

func (builder *Builder) generateOSRel() error {
	osRelease, err := version.OSReleaseFor(version.Name, builder.Version)
	if err != nil {
		return err
	}

	path := filepath.Join(builder.scratchDir, "os-release")

	if err = os.WriteFile(path, osRelease, 0o600); err != nil {
		return err
	}

	builder.sections = append(builder.sections,
		section{
			Name:    secureboot.OSRel,
			Path:    path,
			Measure: true,
			Append:  true,
		},
	)

	return nil
}

func (builder *Builder) generateCmdline() error {
	path := filepath.Join(builder.scratchDir, "cmdline")

	if err := os.WriteFile(path, []byte(builder.Cmdline), 0o600); err != nil {
		return err
	}

	builder.sections = append(builder.sections,
		section{
			Name:    secureboot.CMDLine,
			Path:    path,
			Measure: true,
			Append:  true,
		},
	)

	return nil
}

func (builder *Builder) generateInitrd() error {
	builder.sections = append(builder.sections,
		section{
			Name:    secureboot.Initrd,
			Path:    builder.InitrdPath,
			Measure: true,
			Append:  true,
		},
	)

	return nil
}

func (builder *Builder) generateSplash() error {
	path := filepath.Join(builder.scratchDir, "splash.bmp")

	if err := os.WriteFile(path, splash.GetBootImage(), 0o600); err != nil {
		return err
	}

	builder.sections = append(builder.sections,
		section{
			Name:    secureboot.Splash,
			Path:    path,
			Measure: true,
			Append:  true,
		},
	)

	return nil
}

func (builder *Builder) generateUname() error {
	// it is not always possible to get the kernel version from the kernel image, so we
	// do a bit of pre-checks
	var kernelVersion string

	if builder.Version == version.Tag {
		// if building from the same version of Talos, use default kernel version
		kernelVersion = constants.DefaultKernelVersion
	} else {
		// otherwise, try to get the kernel version from the kernel image
		kernelVersion, _ = DiscoverKernelVersion(builder.KernelPath) //nolint:errcheck
	}

	if kernelVersion == "" {
		// we haven't got the kernel version, skip the uname section
		return nil
	}

	path := filepath.Join(builder.scratchDir, "uname")

	if err := os.WriteFile(path, []byte(kernelVersion), 0o600); err != nil {
		return err
	}

	builder.sections = append(builder.sections,
		section{
			Name:    secureboot.Uname,
			Path:    path,
			Measure: true,
			Append:  true,
		},
	)

	return nil
}

func (builder *Builder) generateSBAT() error {
	sbat, err := GetSBAT(builder.SdStubPath)
	if err != nil {
		return err
	}

	path := filepath.Join(builder.scratchDir, "sbat")

	if err = os.WriteFile(path, sbat, 0o600); err != nil {
		return err
	}

	builder.sections = append(builder.sections,
		section{
			Name:    secureboot.SBAT,
			Path:    path,
			Measure: true,
		},
	)

	return nil
}

func (builder *Builder) generatePCRPublicKey() error {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(builder.PCRSigner.PublicRSAKey())
	if err != nil {
		return err
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  talosx509.PEMTypeRSAPublic,
		Bytes: publicKeyBytes,
	})

	path := filepath.Join(builder.scratchDir, "pcr-public.pem")

	if err = os.WriteFile(path, publicKeyPEM, 0o600); err != nil {
		return err
	}

	builder.sections = append(builder.sections,
		section{
			Name:    secureboot.PCRPKey,
			Path:    path,
			Append:  true,
			Measure: true,
		},
	)

	return nil
}

func (builder *Builder) generateKernel() error {
	path := builder.KernelPath

	if builder.peSigner != nil {
		path := filepath.Join(builder.scratchDir, "kernel")

		if err := builder.peSigner.Sign(builder.KernelPath, path); err != nil {
			return err
		}
	}

	builder.sections = append(builder.sections,
		section{
			Name:    secureboot.Linux,
			Path:    path,
			Append:  true,
			Measure: true,
		},
	)

	return nil
}

func (builder *Builder) generatePCRSig() error {
	sectionsData := xslices.ToMap(
		xslices.Filter(builder.sections,
			func(s section) bool {
				return s.Measure
			},
		),
		func(s section) (secureboot.Section, string) {
			return s.Name, s.Path
		})

	pcrData, err := measure.GenerateSignedPCR(sectionsData, builder.PCRSigner)
	if err != nil {
		return err
	}

	pcrSignatureData, err := json.Marshal(pcrData)
	if err != nil {
		return err
	}

	path := filepath.Join(builder.scratchDir, "pcrpsig")

	if err = os.WriteFile(path, pcrSignatureData, 0o600); err != nil {
		return err
	}

	builder.sections = append(builder.sections,
		section{
			Name:   secureboot.PCRSig,
			Path:   path,
			Append: true,
		},
	)

	return nil
}
