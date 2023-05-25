// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// ukify is a tool to generate UKI bundles from kernel/initramfs...
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "embed"

	"github.com/saferwall/pe"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/ukify/constants"
	"github.com/siderolabs/ukify/measure"

	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	kernelpkg "github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/version"
)

var (
	//go:embed assets/sidero.bmp
	splashBMP []byte
)

var (
	sdStub         string
	sdBoot         string
	kernel         string
	initrd         string
	cmdline        string
	signingKey     string
	signingCert    string
	pcrSigningKey  string
	pcrPublicKey   string
	pcrSigningCert string
	output         string
)

func sbSign(input string) (string, error) {
	out := input + ".signed"

	if err := os.RemoveAll(out); err != nil {
		return "", err
	}

	cmd := exec.Command("sbsign", "--key", signingKey, "--cert", signingCert, "--output", out, input)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	return out, err
}

type section struct {
	name    constants.Section
	file    string
	measure bool
	size    uint32
	vma     uint32
}

func buildUKI(source, output string, sections []section) error {
	peFile, err := pe.New(source, &pe.Options{Fast: true})
	if err != nil {
		return err
	}

	defer peFile.Close() //nolint: errcheck

	if err = peFile.Parse(); err != nil {
		return err
	}

	// find the first VMA address
	lastSection := peFile.Sections[len(peFile.Sections)-1]

	const alignment = 0xfff

	baseVMA := lastSection.Header.VirtualAddress + lastSection.Header.VirtualSize
	baseVMA = (baseVMA + alignment) &^ alignment

	// calculate sections size and VMA
	for i := range sections {
		st, err := os.Stat(sections[i].file)
		if err != nil {
			return err
		}

		sections[i].size = uint32(st.Size())
		sections[i].vma = baseVMA

		baseVMA += sections[i].size
		baseVMA = (baseVMA + alignment) &^ alignment
	}

	// create the output file
	args := []string{}

	for _, section := range sections {
		args = append(args, "--add-section", fmt.Sprintf("%s=%s", section.name, section.file), "--change-section-vma", fmt.Sprintf("%s=0x%x", section.name, section.vma))
	}

	args = append(args, source, output)

	cmd := exec.Command("objcopy", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func Measure(tempDir, kernel, signingKey string, sections []section) ([]section, error) {
	sectionsData := measure.SectionsData{}

	for _, section := range sections {
		if !section.measure {
			continue
		}

		sectionsData[section.name] = section.file
	}

	// manually add the linux section
	sectionsData[constants.Linux] = kernel

	pcrpsigFile := filepath.Join(tempDir, "pcrpsig")

	pcrData, err := measure.GenerateSignedPCR(sectionsData, signingKey)
	if err != nil {
		return nil, err
	}

	pcrSignatureData, err := json.Marshal(&pcrData)
	if err != nil {
		return nil, err
	}

	if err = os.WriteFile(pcrpsigFile, pcrSignatureData, 0644); err != nil {
		return nil, err
	}

	sections = append(sections, section{
		name:    constants.PCRSig,
		file:    pcrpsigFile,
		measure: false,
	})

	return sections, nil
}

func run() error {
	defaultCmdline := procfs.NewCmdline("")
	defaultCmdline.Append(talosconstants.KernelParamPlatform, "metal")

	if err := defaultCmdline.AppendAll(kernelpkg.DefaultArgs); err != nil {
		return err
	}

	flag.StringVar(&sdStub, "sd-stub", "_out/linuxx64.efi.stub", "path to sd-stub")
	flag.StringVar(&sdBoot, "sd-boot", "_out/systemd-bootx64.efi", "path to sd-boot")
	flag.StringVar(&output, "output", "_out/vmlinuz.efi", "output path")
	flag.StringVar(&kernel, "kernel", "_out/vmlinuz-amd64", "path to kernel image")
	flag.StringVar(&initrd, "initrd", "_out/initramfs-amd64.xz", "path to initrd image")
	flag.StringVar(&cmdline, "cmdline", defaultCmdline.String(), "kernel cmdline")
	flag.StringVar(&signingKey, "signing-key-path", "_out/uki-certs/uki-signing-key.pem", "path to signing key")
	flag.StringVar(&signingCert, "signing-cert-path", "_out/uki-certs/uki-signing-cert.pem", "path to signing cert")
	flag.StringVar(&pcrSigningKey, "pcr-signing-key-path", "_out/uki-certs/pcr-signing-key.pem", "path to PCR signing key")
	flag.StringVar(&pcrPublicKey, "pcr-public-key-path", "_out/uki-certs/pcr-signing-public-key.pem", "path to PCR public key")
	flag.StringVar(&pcrSigningCert, "prc-signing-cert-path", "_out/uki-certs/pcr-signing-cert.pem", "path to PCR signing cert")
	flag.Parse()

	_, err := sbSign(sdBoot)
	if err != nil {
		return fmt.Errorf("failed to sign sd-boot: %w", err)
	}

	signedKernel, err := sbSign(kernel)
	if err != nil {
		return fmt.Errorf("failed to sign kernel: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "ukify")
	if err != nil {
		return err
	}

	defer func() {
		if err = os.RemoveAll(tempDir); err != nil {
			log.Printf("failed to remove temp dir: %v", err)
		}
	}()

	cmdlineFile := filepath.Join(tempDir, "cmdline")

	if err = os.WriteFile(cmdlineFile, []byte(cmdline), 0o644); err != nil {
		return err
	}

	unameFile := filepath.Join(tempDir, "uname")

	if err = os.WriteFile(unameFile, []byte(talosconstants.DefaultKernelVersion), 0o644); err != nil {
		return err
	}

	osReleaseFile := filepath.Join(tempDir, "os-release")

	var buf bytes.Buffer

	tmpl, err := template.New("").Parse(talosconstants.OSReleaseTemplate)
	if err != nil {
		return err
	}

	if err = tmpl.Execute(&buf, struct {
		Name    string
		ID      string
		Version string
	}{
		Name:    version.Name,
		ID:      strings.ToLower(version.Name),
		Version: version.Tag,
	}); err != nil {
		return err
	}

	if err = os.WriteFile(osReleaseFile, buf.Bytes(), 0o644); err != nil {
		return err
	}

	splashFile := filepath.Join(tempDir, "splash.bmp")

	if err = os.WriteFile(splashFile, splashBMP, 0o644); err != nil {
		return err
	}

	sections := []section{
		{
			name:    constants.OSRel,
			file:    osReleaseFile,
			measure: true,
		},
		{
			name:    constants.CMDLine,
			file:    cmdlineFile,
			measure: true,
		},
		{
			name:    constants.Initrd,
			file:    initrd,
			measure: true,
		},
		{
			name:    constants.Splash,
			file:    splashFile,
			measure: true,
		},
		{
			name:    constants.Uname,
			file:    unameFile,
			measure: true,
		},
		{
			name:    constants.PCRPKey,
			file:    pcrPublicKey,
			measure: true,
		},
	}

	// systemd-measure
	if sections, err = Measure(tempDir, signedKernel, pcrSigningKey, sections); err != nil {
		return err
	}

	// kernel is added last to account for decompression
	sections = append(sections,
		section{
			name:    constants.Linux,
			file:    signedKernel,
			measure: true,
		},
	)

	if err = os.RemoveAll(output); err != nil {
		return err
	}

	return buildUKI(sdStub, output, sections)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
