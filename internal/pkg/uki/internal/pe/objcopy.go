// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pe

import (
	"context"
	"debug/pe"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// AssembleObjcopy is a helper function to assemble the PE file using objcopy.
func AssembleObjcopy(ctx context.Context, srcPath, dstPath string, sections []Section) error {
	peFile, err := pe.Open(srcPath)
	if err != nil {
		return err
	}

	defer peFile.Close() //nolint: errcheck

	// find the first VMA address
	lastSection := peFile.Sections[len(peFile.Sections)-1]

	header, ok := peFile.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		return errors.New("failed to get optional header")
	}

	sectionAlignment := uint64(header.SectionAlignment - 1)

	baseVMA := header.ImageBase + uint64(lastSection.VirtualAddress) + uint64(lastSection.VirtualSize)
	baseVMA = (baseVMA + sectionAlignment) &^ sectionAlignment

	// calculate sections size and VMA
	for i := range sections {
		if !sections[i].Append {
			continue
		}

		st, err := os.Stat(sections[i].Path)
		if err != nil {
			return err
		}

		sections[i].virtualSize = uint64(st.Size())
		sections[i].virtualAddress = baseVMA

		baseVMA += sections[i].virtualSize
		baseVMA = (baseVMA + sectionAlignment) &^ sectionAlignment
	}

	// create the output file
	args := make([]string, 0, len(sections)+2)

	for _, section := range sections {
		if !section.Append {
			continue
		}

		args = append(args, "--add-section", fmt.Sprintf("%s=%s", section.Name, section.Path), "--change-section-vma", fmt.Sprintf("%s=0x%x", section.Name, section.virtualAddress))
	}

	args = append(args, srcPath, dstPath)

	cmd := exec.CommandContext(ctx, "objcopy", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
