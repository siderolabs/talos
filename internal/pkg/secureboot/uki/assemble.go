// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uki

import (
	"debug/pe"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// assemble the UKI file out of sections.
func (builder *Builder) assemble() error {
	peFile, err := pe.Open(builder.SdStubPath)
	if err != nil {
		return err
	}

	defer peFile.Close() //nolint: errcheck

	// find the first VMA address
	lastSection := peFile.Sections[len(peFile.Sections)-1]

	// align the VMA to 512 bytes
	// https://github.com/saferwall/pe/blob/main/helper.go#L22-L26
	const alignment = 0x1ff

	header, ok := peFile.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		return errors.New("failed to get optional header")
	}

	baseVMA := header.ImageBase + uint64(lastSection.VirtualAddress) + uint64(lastSection.VirtualSize)
	baseVMA = (baseVMA + alignment) &^ alignment

	// calculate sections size and VMA
	for i := range builder.sections {
		if !builder.sections[i].Append {
			continue
		}

		st, err := os.Stat(builder.sections[i].Path)
		if err != nil {
			return err
		}

		builder.sections[i].Size = uint64(st.Size())
		builder.sections[i].VMA = baseVMA

		baseVMA += builder.sections[i].Size
		baseVMA = (baseVMA + alignment) &^ alignment
	}

	// create the output file
	args := make([]string, 0, len(builder.sections)+2)

	for _, section := range builder.sections {
		if !section.Append {
			continue
		}

		args = append(args, "--add-section", fmt.Sprintf("%s=%s", section.Name, section.Path), "--change-section-vma", fmt.Sprintf("%s=0x%x", section.Name, section.VMA))
	}

	builder.unsignedUKIPath = filepath.Join(builder.scratchDir, "unsigned.uki")

	args = append(args, builder.SdStubPath, builder.unsignedUKIPath)

	objcopy := "/usr/x86_64-alpine-linux-musl/bin/objcopy"

	if builder.Arch == "arm64" {
		objcopy = "/usr/aarch64-alpine-linux-musl/bin/objcopy"
	}

	cmd := exec.Command(objcopy, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
