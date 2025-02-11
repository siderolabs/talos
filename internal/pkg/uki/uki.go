// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package uki creates the UKI file out of the sd-stub and other sections.
package uki

import (
	"fmt"
	"log"
	"os"

	"github.com/siderolabs/talos/internal/pkg/measure"
	"github.com/siderolabs/talos/internal/pkg/secureboot/pesign"
	"github.com/siderolabs/talos/internal/pkg/uki/internal/pe"
	"github.com/siderolabs/talos/pkg/imager/utils"
)

// Section is a name of a PE file section (UEFI binary).
type Section string

// List of well-known section names.
const (
	SectionLinux   Section = ".linux"
	SectionOSRel   Section = ".osrel"
	SectionCmdline Section = ".cmdline"
	SectionInitrd  Section = ".initrd"
	SectionUcode   Section = ".ucode"
	SectionSplash  Section = ".splash"
	SectionDTB     Section = ".dtb"
	SectionUname   Section = ".uname"
	SectionSBAT    Section = ".sbat"
	SectionPCRSig  Section = ".pcrsig"
	SectionPCRPKey Section = ".pcrpkey"
	SectionProfile Section = ".profile"
	SectionDTBAuto Section = ".dtbauto"
	SectionHWIDS   Section = ".hwids"
)

// String returns the string representation of the section.
func (s Section) String() string {
	return string(s)
}

type section = pe.Section

// Builder is a UKI file builder.
type Builder struct {
	// Source options.
	//
	// Arch of the UKI file.
	Arch string
	// Version of Talos.
	Version string
	// Path to the sd-stub.
	SdStubPath string
	// Path to the sd-boot.
	SdBootPath string
	// Path to the kernel image.
	KernelPath string
	// Path to the initrd image.
	InitrdPath string
	// Kernel cmdline.
	Cmdline string
	// SecureBoot certificate and signer.
	SecureBootSigner pesign.CertificateSigner
	// PCR signer.
	PCRSigner measure.RSAKey
	// Profiles to include in the UKI.
	Profiles []Profile

	// Output options:
	//
	// Path to the signed sd-boot.
	OutSdBootPath string
	// Path to the output UKI file.
	OutUKIPath string

	// fields initialized during build
	sections        []section
	scratchDir      string
	peSigner        *pesign.Signer
	unsignedUKIPath string
}

// Profile is a UKI Profile.
// For now only cmdline is supported.
type Profile struct {
	ID    string
	Title string

	Cmdline string
}

// String returns the string representation of the profile that gets adds to the `.profile` section.
func (p Profile) String() string {
	s := fmt.Sprintf("ID=%s", p.ID)

	if p.Title != "" {
		s += fmt.Sprintf("\nTITLE=%s", p.Title)
	}

	return s
}

// Build the unsigned UKI file.
//
// Build process is as follows:
//   - build ephemeral sections (uname, os-release), and other proposed sections
//   - assemble the final UKI file starting from sd-stub and appending generated section.
func (builder *Builder) Build(printf func(string, ...any)) error {
	var err error

	builder.scratchDir, err = os.MkdirTemp("", "talos-uki")
	if err != nil {
		return err
	}

	defer func() {
		if err = os.RemoveAll(builder.scratchDir); err != nil {
			log.Printf("failed to remove scratch dir: %v", err)
		}
	}()

	if err := utils.CopyFiles(printf, utils.SourceDestination(builder.SdBootPath, builder.OutSdBootPath)); err != nil {
		return err
	}

	printf("generating UKI sections")

	// generate and build list of all sections
	for _, generateSection := range []func() error{
		builder.generateSBAT,
		builder.generateOSRel,
		builder.generateCmdline,
		builder.generateUname,
		builder.generateSplash,
		builder.generateKernel,
		builder.generateInitrd,
		builder.generateProfiles,
	} {
		if err = generateSection(); err != nil {
			return fmt.Errorf("error generating sections: %w", err)
		}
	}

	printf("assembling UKI")

	// assemble the final UKI file
	if err = builder.assemble(); err != nil {
		return fmt.Errorf("error assembling UKI: %w", err)
	}

	return utils.CopyFiles(printf, utils.SourceDestination(builder.unsignedUKIPath, builder.OutUKIPath))
}

// BuildSigned the UKI file.
//
// BuildSigned process is as follows:
//   - sign the sd-boot EFI binary, and write it to the OutSdBootPath
//   - build ephemeral sections (uname, os-release), and other proposed sections
//   - measure sections, generate signature, and append to the list of sections
//   - assemble the final UKI file starting from sd-stub and appending generated section.
func (builder *Builder) BuildSigned(printf func(string, ...any)) error {
	var err error

	builder.scratchDir, err = os.MkdirTemp("", "talos-uki")
	if err != nil {
		return err
	}

	defer func() {
		if err = os.RemoveAll(builder.scratchDir); err != nil {
			log.Printf("failed to remove scratch dir: %v", err)
		}
	}()

	printf("signing systemd-boot")

	builder.peSigner, err = pesign.NewSigner(builder.SecureBootSigner)
	if err != nil {
		return fmt.Errorf("error initializing signer: %w", err)
	}

	// sign sd-boot
	if err = builder.peSigner.Sign(builder.SdBootPath, builder.OutSdBootPath); err != nil {
		return fmt.Errorf("error signing sd-boot: %w", err)
	}

	printf("generating UKI sections")

	// generate and build list of all sections
	for _, generateSection := range []func() error{
		builder.generateSBAT,
		builder.generateOSRel,
		builder.generateCmdline,
		builder.generateUname,
		builder.generateSplash,
		builder.generatePCRPublicKey,
		builder.generateKernel,
		builder.generateInitrd,
		builder.generateProfiles,
		builder.generatePCRSig,
	} {
		if err = generateSection(); err != nil {
			return fmt.Errorf("error generating sections: %w", err)
		}
	}

	printf("assembling UKI")

	// assemble the final UKI file
	if err = builder.assemble(); err != nil {
		return fmt.Errorf("error assembling UKI: %w", err)
	}

	printf("signing UKI")

	// sign the UKI file
	return builder.peSigner.Sign(builder.unsignedUKIPath, builder.OutUKIPath)
}

// Extract extracts the kernel, initrd, and cmdline from the UKI file.
func Extract(ukiPath string) (asset pe.AssetInfo, err error) {
	return pe.Extract(ukiPath)
}
