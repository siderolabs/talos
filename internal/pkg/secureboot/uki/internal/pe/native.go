// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pe

import (
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/pkg/imager/utils"
)

const (
	dosHeaderLength  = 0x40
	dosHeaderPadding = 0x40
)

// AssembleNative is a helper function to assemble the PE file without external programs.
//
//nolint:gocyclo,cyclop
func AssembleNative(srcPath, dstPath string, sections []Section) error {
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	peFile, err := pe.NewFile(in)
	if err != nil {
		return err
	}

	defer in.Close() //nolint: errcheck

	if peFile.COFFSymbols != nil {
		return errors.New("COFF symbols are not supported")
	}

	if peFile.StringTable != nil {
		return errors.New("COFF string table is not supported")
	}

	if peFile.Symbols != nil {
		return errors.New("symbols are not supported")
	}

	_, err = in.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}

	defer out.Close() //nolint: errcheck

	// 1. DOS header
	var dosheader [dosHeaderLength]byte

	_, err = in.ReadAt(dosheader[:], 0)
	if err != nil {
		return fmt.Errorf("failed to read DOS header: %w", err)
	}

	binary.LittleEndian.PutUint32(dosheader[dosHeaderLength-4:], dosHeaderLength+dosHeaderPadding)

	_, err = out.Write(append(append(dosheader[:], make([]byte, dosHeaderPadding)...), []byte("PE\x00\x00")...))
	if err != nil {
		return fmt.Errorf("failed to write DOS header: %w", err)
	}

	// 2. PE header and optional header
	newFileHeader := peFile.FileHeader

	timestamp, ok, err := utils.SourceDateEpoch()
	if err != nil {
		return fmt.Errorf("failed to get SOURCE_DATE_EPOCH: %w", err)
	}

	if !ok {
		timestamp = time.Now().Unix()
	}

	newFileHeader.TimeDateStamp = uint32(timestamp)

	// find the first VMA address
	lastSection := peFile.Sections[len(peFile.Sections)-1]

	header, ok := peFile.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		return errors.New("failed to get optional header")
	}

	sectionAlignment := uint64(header.SectionAlignment - 1)
	fileAlignment := uint64(header.FileAlignment - 1)

	baseVirtualAddress := uint64(lastSection.VirtualAddress) + uint64(lastSection.VirtualSize)
	baseVirtualAddress = (baseVirtualAddress + sectionAlignment) &^ sectionAlignment

	newHeader := *header
	newHeader.MajorLinkerVersion = 0
	newHeader.MinorLinkerVersion = 0
	newHeader.CheckSum = 0

	newSections := slices.Clone(peFile.Sections)

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
		sections[i].virtualAddress = baseVirtualAddress

		baseVirtualAddress += sections[i].virtualSize
		baseVirtualAddress = (baseVirtualAddress + sectionAlignment) &^ sectionAlignment

		newFileHeader.NumberOfSections++

		newSections = append(newSections, &pe.Section{
			SectionHeader: pe.SectionHeader{
				Name:            string(sections[i].Name),
				VirtualSize:     uint32(sections[i].virtualSize),
				VirtualAddress:  uint32(sections[i].virtualAddress),
				Size:            uint32((sections[i].virtualSize + fileAlignment) &^ fileAlignment),
				Characteristics: pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ,
			},
		})
	}

	newHeader.SizeOfInitializedData = 0
	newHeader.SizeOfCode = 0
	newHeader.SizeOfHeaders = 0x80 /* DOS header */ + uint32(binary.Size(pe.FileHeader{})+binary.Size(pe.OptionalHeader64{})+binary.Size(pe.SectionHeader32{})*len(newSections))
	newHeader.SizeOfHeaders = (newHeader.SizeOfHeaders + uint32(fileAlignment)) &^ uint32(fileAlignment)

	lastNewSection := newSections[len(newSections)-1]

	lastSectionPointer := uint64(lastNewSection.VirtualAddress) + uint64(lastNewSection.VirtualSize) + newHeader.ImageBase
	lastSectionPointer = (lastSectionPointer + sectionAlignment) &^ sectionAlignment

	newHeader.SizeOfImage = uint32(lastSectionPointer - newHeader.ImageBase)

	for _, section := range newSections {
		if section.Characteristics&pe.IMAGE_SCN_CNT_INITIALIZED_DATA != 0 {
			newHeader.SizeOfInitializedData += section.Size
		} else {
			newHeader.SizeOfCode += section.Size
		}
	}

	// write the new file header
	if err = binary.Write(out, binary.LittleEndian, newFileHeader); err != nil {
		return fmt.Errorf("failed to write file header: %w", err)
	}

	if err = binary.Write(out, binary.LittleEndian, newHeader); err != nil {
		return fmt.Errorf("failed to write optional header: %w", err)
	}

	// 3. Section headers
	rawSections := xslices.Map(newSections, func(section *pe.Section) pe.SectionHeader32 {
		var rawName [8]byte

		copy(rawName[:], section.Name)

		return pe.SectionHeader32{
			Name:            rawName,
			VirtualSize:     section.VirtualSize,
			VirtualAddress:  section.VirtualAddress,
			SizeOfRawData:   section.Size,
			Characteristics: section.Characteristics,
		}
	},
	)

	sectionOffset := newHeader.SizeOfHeaders

	for i := range rawSections {
		rawSections[i].PointerToRawData = sectionOffset

		sectionOffset += rawSections[i].SizeOfRawData
	}

	for _, rawSection := range rawSections {
		if err = binary.Write(out, binary.LittleEndian, rawSection); err != nil {
			return fmt.Errorf("failed to write section header: %w", err)
		}
	}

	// 4. Section data
	for i, rawSection := range rawSections {
		name := newSections[i].Name

		if err := func(rawSection pe.SectionHeader32, name string) error {
			// the section might come either from the input PE file or from a separate file
			var sectionData io.ReadCloser

			for _, section := range sections {
				if section.Append && section.Name == secureboot.Section(name) {
					sectionData, err = os.Open(section.Path)
					if err != nil {
						return fmt.Errorf("failed to open section data: %w", err)
					}

					defer sectionData.Close() //nolint: errcheck

					break
				}
			}

			if sectionData == nil {
				for _, section := range peFile.Sections {
					if section.Name == name {
						sectionData = io.NopCloser(section.Open())

						break
					}
				}
			}

			if sectionData == nil {
				return fmt.Errorf("failed to find section data for %q", name)
			}

			_, err = out.Seek(int64(rawSection.PointerToRawData), io.SeekStart)
			if err != nil {
				return fmt.Errorf("failed to seek to section data: %w", err)
			}

			n, err := io.Copy(out, sectionData)
			if err != nil {
				return fmt.Errorf("failed to copy section data: %w", err)
			}

			if n > int64(rawSection.SizeOfRawData) {
				return fmt.Errorf("section data is too large: %d > %d", n, rawSection.SizeOfRawData)
			}

			if n < int64(rawSection.SizeOfRawData) {
				_, err = io.CopyN(out, zeroReader{}, int64(rawSection.SizeOfRawData)-n)
				if err != nil {
					return fmt.Errorf("failed to zero-fill section data: %w", err)
				}
			}

			return nil
		}(rawSection, name); err != nil {
			return fmt.Errorf("failed to write section data %s: %w", name, err)
		}
	}

	return nil
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}

	return len(p), nil
}
