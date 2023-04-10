// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// main is the entrypoint for the program.
package main

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"go.mozilla.org/pkcs7"
)

// Reverse engineered from the kernel source code: https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/scripts/extract-module-sig.pl
// Ref:
// * https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/Documentation/admin-guide/module-signing.rst
// * https://unix.stackexchange.com/questions/493170/how-to-verify-a-kernel-module-signature
// * https://wiki.gentoo.org/wiki/Signed_kernel_module_support
//
// A signed kernel module has the following structure:
// * module data in ELF format followed by
// * 12 bytes of signature info (https://github.com/torvalds/linux/blob/master/scripts/sign-file.c#L62-L70) followed by
// * the signature itself followed by
// * the magic string "~Module signature appended~\n"

const (
	// SignedModuleMagic is the magic string appended to the end of a signed module.
	SignedModuleMagic = "~Module signature appended~\n"
	// ModuleSignatureInfoLength is the length of the signature info.
	ModuleSignatureInfoLength = 12
)

var (
	cert   string
	module string
)

func main() {
	flag.StringVar(&cert, "cert", "", "X.509 certificate used to sign the module")
	flag.StringVar(&module, "module", "", "path to the module to verify, if '-' is passed, the module data will be read from stdin")
	flag.Parse()

	if cert == "" || module == "" {
		flag.Usage()
		os.Exit(1)
	}

	cert, err := os.ReadFile(cert)
	if err != nil {
		fmt.Printf("failed to read certificate file %s: %v", cert, err)
		os.Exit(1)
	}

	crt, err := x509.ParseCertificate(cert)
	if err != nil {
		fmt.Printf("failed to parse certificate file %s: %v", cert, err)
		os.Exit(1)
	}

	moduleData, err := parseModuleInput(module)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := verifyModule(crt, moduleData); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func parseModuleInput(module string) (*bytes.Reader, error) {
	if module == "-" {
		moduleData, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read module from stdin: %w", err)
		}

		return bytes.NewReader(moduleData), nil
	}

	moduleData, err := os.ReadFile(module)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", module, err)
	}

	return bytes.NewReader(moduleData), nil
}

func verifyModule(crt *x509.Certificate, moduleData *bytes.Reader) error {
	fileLen := moduleData.Size()
	signedModuleMagicStart := fileLen - int64(len(SignedModuleMagic))

	_, err := moduleData.Seek(signedModuleMagicStart, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to %d in file %s: %w", signedModuleMagicStart, module, err)
	}

	magicBytes := make([]byte, len(SignedModuleMagic))

	_, err = moduleData.Read(magicBytes)
	if err != nil {
		return fmt.Errorf("failed to read %d bytes from file %s: %w", len(SignedModuleMagic), module, err)
	}

	if string(magicBytes) != SignedModuleMagic {
		return fmt.Errorf("file %s is not a signed module", module)
	}

	signatureInfoStart := signedModuleMagicStart - ModuleSignatureInfoLength

	_, err = moduleData.Seek(signatureInfoStart, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to %d in file %s: %w", signatureInfoStart, module, err)
	}

	signatureBytes := make([]byte, ModuleSignatureInfoLength)

	_, err = moduleData.Read(signatureBytes)
	if err != nil {
		return fmt.Errorf("failed to read %d bytes from file %s: %w", ModuleSignatureInfoLength, module, err)
	}

	// The signature length is encoded in the last 2 bytes of the signature info.
	signatureLength, err := strconv.ParseInt(hex.EncodeToString(signatureBytes[(len(signatureBytes)-2):]), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse signature length %w", err)
	}

	signatureStart := signatureInfoStart - signatureLength

	_, err = moduleData.Seek(signatureStart, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to %d in file %s: %w", signatureStart, module, err)
	}

	signature := make([]byte, signatureLength)

	_, err = moduleData.Read(signature)
	if err != nil {
		return fmt.Errorf("failed to read %d bytes from file %s: %w", signatureLength, module, err)
	}

	moduleWithSignatureLength := fileLen - int64(len(SignedModuleMagic))

	_, err = moduleData.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to %d in file %s: %w", 0, module, err)
	}

	moduleWithSignature := make([]byte, moduleWithSignatureLength)

	_, err = moduleData.Read(moduleWithSignature)
	if err != nil {
		return fmt.Errorf("failed to read %d bytes from file %s: %w", moduleWithSignatureLength, module, err)
	}

	_, err = moduleData.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to %d in file %s: %w", 0, module, err)
	}

	unsignedModuleLength := fileLen - signatureLength - ModuleSignatureInfoLength - int64(len(SignedModuleMagic))
	unsignedModuleData := make([]byte, unsignedModuleLength)

	_, err = moduleData.Read(unsignedModuleData)
	if err != nil {
		return fmt.Errorf("failed to read %d bytes from file %s: %w", unsignedModuleLength, module, err)
	}

	p7, err := pkcs7.Parse(signature)
	if err != nil {
		return fmt.Errorf("failed to parse signature: %w", err)
	}

	signatureSigned := p7.Signers[0].EncryptedDigest
	hashed := sha512.Sum512(unsignedModuleData)

	pubKey, ok := crt.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("failed to convert public key to RSA key")
	}

	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA512, hashed[:], signatureSigned); err != nil {
		return fmt.Errorf("failed to verify signature for module %s", module)
	}

	fmt.Printf("module %s is signed by %s\n", module, crt.Subject.CommonName)

	return nil
}
