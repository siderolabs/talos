// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uki

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	talosx509 "github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/pkg/measure"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
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
			Name:    SectionOSRel.String(),
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
			Name:    SectionCmdline.String(),
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
			Name:    SectionInitrd.String(),
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
			Name:    SectionSplash.String(),
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
			Name:    SectionUname.String(),
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
			Name:    SectionSBAT.String(),
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
			Name:    SectionPCRPKey.String(),
			Path:    path,
			Append:  true,
			Measure: true,
		},
	)

	return nil
}

func (builder *Builder) generateProfiles() error {
	if !quirks.New(builder.Version).SupportsUKIProfiles() {
		return nil
	}

	for _, profile := range []Profile{
		{
			ID: "main",
		},
		{
			ID:    "reset-maintenance",
			Title: "Reset to maintenance mode",

			Cmdline: builder.Cmdline + " talos.experimental.wipe=system:EPHEMERAL,STATE",
		},
		{
			ID:      "reset",
			Title:   "Reset system disk",
			Cmdline: builder.Cmdline + " talos.experimental.wipe=system",
		},
	} {
		path := filepath.Join(builder.scratchDir, fmt.Sprintf("profile-%s", profile.ID))

		if err := os.WriteFile(path, []byte(profile.String()), 0o600); err != nil {
			return err
		}

		builder.sections = append(builder.sections,
			section{
				Name:    SectionProfile.String(),
				Path:    path,
				Append:  true,
				Measure: true,
			},
		)

		if profile.Cmdline != "" {
			path = filepath.Join(builder.scratchDir, fmt.Sprintf("profile-%s-cmdline", profile.ID))

			if err := os.WriteFile(path, []byte(profile.Cmdline), 0o600); err != nil {
				return err
			}

			builder.sections = append(builder.sections,
				section{
					Name:    SectionCmdline.String(),
					Path:    path,
					Append:  true,
					Measure: true,
				},
			)
		}
	}

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
			Name:    SectionLinux.String(),
			Path:    path,
			Append:  true,
			Measure: true,
		},
	)

	return nil
}

type profileIndex struct {
	Start int
	End   int
}

func (builder *Builder) generatePCRSig() error {
	toMeasure := xslices.Filter(builder.sections, func(s section) bool {
		return s.Measure
	})

	if quirks.New(builder.Version).SupportsUKIProfiles() {
		profileIndexes := []profileIndex{}

		var previousProfileIndex int

		for i, s := range toMeasure {
			if s.Name == SectionProfile.String() {
				if previousProfileIndex != 0 {
					profileIndexes[len(profileIndexes)-1].End = i
				}

				profileIndexes = append(profileIndexes, profileIndex{Start: i})
				previousProfileIndex = i
			}
		}

		if previousProfileIndex != 0 {
			profileIndexes[len(profileIndexes)-1].End = len(toMeasure)
		}

		for i, profileIndex := range profileIndexes {
			profileData := xslices.ToMap(
				slices.Concat(toMeasure[:profileIndexes[0].Start], toMeasure[profileIndex.Start:profileIndex.End]),
				func(s section) (string, string) {
					return s.Name, s.Path
				},
			)

			if err := builder.writePCRSignature(profileData, fmt.Sprintf("pcrpsig-%d", i), profileIndex.End+i); err != nil {
				return err
			}
		}

		return nil
	}

	sectionData := xslices.ToMap(toMeasure, func(s section) (string, string) {
		return s.Name, s.Path
	})

	return builder.writePCRSignature(sectionData, "pcrpsig", len(builder.sections))
}

func (builder *Builder) writePCRSignature(data map[string]string, filename string, insertIndex int) error {
	pcrData, err := measure.GenerateSignedPCR(data, builder.PCRSigner)
	if err != nil {
		return err
	}

	pcrSignatureData, err := json.Marshal(pcrData)
	if err != nil {
		return err
	}

	path := filepath.Join(builder.scratchDir, filename)

	if err = os.WriteFile(path, pcrSignatureData, 0o600); err != nil {
		return err
	}

	builder.sections = slices.Insert(builder.sections, insertIndex, section{
		Name:   SectionPCRSig.String(),
		Path:   path,
		Append: true,
	})

	return nil
}
