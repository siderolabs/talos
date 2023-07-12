// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main provides the ukfiy implementation.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/internal/pkg/secureboot/uki"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	kernelpkg "github.com/siderolabs/talos/pkg/machinery/kernel"
)

// NOTE: this is temporary implementation, it will be moved to the imager
//       in the next round of refactoring.

func run() error {
	metal, err := platform.NewPlatform("metal")
	if err != nil {
		return fmt.Errorf("failed to create platform: %w", err)
	}

	defaultCmdline := procfs.NewCmdline("")
	defaultCmdline.Append(constants.KernelParamPlatform, "metal")

	if err := defaultCmdline.AppendAll(kernelpkg.DefaultArgs); err != nil {
		return err
	}

	if err := defaultCmdline.AppendAll(metal.KernelArgs().Strings()); err != nil {
		return err
	}

	var builder uki.Builder

	flag.StringVar(&builder.SdStubPath, "sd-stub", "_out/linuxx64.efi.stub", "path to sd-stub")
	flag.StringVar(&builder.SdBootPath, "sd-boot", "_out/systemd-bootx64.efi", "path to sd-boot")
	flag.StringVar(&builder.KernelPath, "kernel", "_out/vmlinuz-amd64", "path to kernel image")
	flag.StringVar(&builder.InitrdPath, "initrd", "_out/initramfs-amd64.xz", "path to initrd image")
	flag.StringVar(&builder.Cmdline, "cmdline", defaultCmdline.String(), "kernel cmdline")
	flag.StringVar(&builder.SigningKeyPath, "signing-key-path", "_out/uki-certs/uki-signing-key.pem", "path to signing key")
	flag.StringVar(&builder.SigningCertPath, "signing-cert-path", "_out/uki-certs/uki-signing-cert.pem", "path to signing cert")
	flag.StringVar(&builder.PCRSigningKeyPath, "pcr-signing-key-path", "_out/uki-certs/pcr-signing-key.pem", "path to PCR signing key")
	flag.StringVar(&builder.PCRPublicKeyPath, "pcr-public-key-path", "_out/uki-certs/pcr-signing-public-key.pem", "path to PCR public key")

	flag.StringVar(&builder.OutUKIPath, "output", "_out/vmlinuz.efi.signed", "output path")
	flag.StringVar(&builder.OutSdBootPath, "sdboot-output", "_out/systemd-boot.efi.signed", "output path")
	flag.Parse()

	return builder.Build()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
