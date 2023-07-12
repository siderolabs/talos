// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/foxboron/go-uefi/efi"
	"github.com/foxboron/go-uefi/efi/signature"
	"github.com/foxboron/go-uefi/efi/util"
	"github.com/google/uuid"
	"github.com/siderolabs/crypto/x509"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
)

var genSecurebootCmdFlags struct {
	outputDirectory string
}

// genSecurebootCmd represents the `gen secureboot` command.
var genSecurebootCmd = &cobra.Command{
	Use:   "secureboot",
	Short: "Generates secrets for the SecureBoot process",
	Long:  ``,
}

var genSecurebootUKICmdFlags struct {
	commonName string
}

// genSecurebootUKICmd represents the `gen secureboot uki` command.
var genSecurebootUKICmd = &cobra.Command{
	Use:   "uki",
	Short: "Generates a certificate which is used to sign boot assets (UKI)",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return generateSigningCerts(genSecurebootCmdFlags.outputDirectory, "uki", genSecurebootUKICmdFlags.commonName, 4096, true)
	},
}

// genSecurebootPCRCmd represents the `gen secureboot pcr` command.
var genSecurebootPCRCmd = &cobra.Command{
	Use:   "pcr",
	Short: "Generates a key which is used to sign TPM PCR values",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return generateSigningCerts(genSecurebootCmdFlags.outputDirectory, "pcr", "dummy", 2048, false)
	},
}

var genSecurebootDatabaseCmdFlags struct {
	enrolledCertificatePath                string
	signingCertificatePath, signingKeyPath string
}

// genSecurebootDatabaseCmd represents the `gen secureboot database` command.
var genSecurebootDatabaseCmd = &cobra.Command{
	Use:   "database",
	Short: "Generates a UEFI database to enroll the signing certificate",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return generateSecureBootDatabase(
			genSecurebootCmdFlags.outputDirectory,
			genSecurebootDatabaseCmdFlags.enrolledCertificatePath,
			genSecurebootDatabaseCmdFlags.signingKeyPath,
			genSecurebootDatabaseCmdFlags.signingCertificatePath,
		)
	},
}

func checkedWrite(path string, data []byte, perm fs.FileMode) error { //nolint:unparam
	if err := validateFileExists(path); err != nil {
		return err
	}

	if dirname := filepath.Dir(path); dirname != "." {
		if err := os.MkdirAll(dirname, 0o700); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "writing %s\n", path)

	return os.WriteFile(path, data, perm)
}

func generateSigningCerts(path, prefix, commonName string, rsaBits int, outputCert bool) error {
	currentTime := time.Now()

	opts := []x509.Option{
		x509.RSA(true),
		x509.Bits(rsaBits),
		x509.CommonName(commonName),
		x509.NotAfter(currentTime.Add(secrets.CAValidityTime)),
		x509.NotBefore(currentTime),
		x509.Organization(commonName),
	}

	signingKey, err := x509.NewSelfSignedCertificateAuthority(opts...)
	if err != nil {
		return err
	}

	if outputCert {
		if err = checkedWrite(filepath.Join(path, prefix+"-signing-cert.pem"), signingKey.CrtPEM, 0o600); err != nil {
			return err
		}
	}

	if err = checkedWrite(filepath.Join(path, prefix+"-signing-key.pem"), signingKey.KeyPEM, 0o600); err != nil {
		return err
	}

	if !outputCert {
		pemKey := x509.PEMEncodedKey{
			Key: signingKey.KeyPEM,
		}

		privKey, err := pemKey.GetRSAKey()
		if err != nil {
			return err
		}

		if err = checkedWrite(filepath.Join(path, prefix+"-signing-public-key.pem"), privKey.PublicKeyPEM, 0o600); err != nil {
			return err
		}
	}

	return nil
}

// generateSecureBootDatabase generates a UEFI database to enroll the signing certificate.
//
// ref: https://blog.hansenpartnership.com/the-meaning-of-all-the-uefi-keys/
//
//nolint:gocyclo
func generateSecureBootDatabase(path, enrolledCertificatePath, signingKeyPath, signingCertificatePath string) error {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	efiGUID := util.StringToGUID(uuid.String())

	// Reuse the generated test signing key for secure boot
	signingPEM, err := x509.NewCertificateAndKeyFromFiles(signingCertificatePath, signingKeyPath)
	if err != nil {
		return err
	}

	cert, err := signingPEM.GetCert()
	if err != nil {
		return err
	}

	key, err := signingPEM.GetRSAKey()
	if err != nil {
		return err
	}

	enrolledPEM, err := os.ReadFile(enrolledCertificatePath)
	if err != nil {
		return err
	}

	// Create ESL
	db := signature.NewSignatureDatabase()
	if err = db.Append(signature.CERT_X509_GUID, *efiGUID, enrolledPEM); err != nil {
		return err
	}

	// Sign the ESL, but for each EFI variable
	signedDB, err := efi.SignEFIVariable(key, cert, "db", db.Bytes())
	if err != nil {
		return err
	}

	signedKEK, err := efi.SignEFIVariable(key, cert, "KEK", db.Bytes())
	if err != nil {
		return err
	}

	signedPK, err := efi.SignEFIVariable(key, cert, "PK", db.Bytes())
	if err != nil {
		return err
	}

	// output all files with sd-boot conventional names for auto-enrolment
	for _, out := range []struct {
		name string
		data []byte
	}{
		{"db.auth", signedDB},
		{"KEK.auth", signedKEK},
		{"PK.auth", signedPK},
	} {
		if err = checkedWrite(filepath.Join(path, out.name), out.data, 0o600); err != nil {
			return err
		}
	}

	return nil
}

func init() {
	genSecurebootCmd.PersistentFlags().StringVarP(&genSecurebootCmdFlags.outputDirectory, "output", "o", "_out", "path to the directory storing the generated files")
	Cmd.AddCommand(genSecurebootCmd)

	genSecurebootUKICmd.Flags().StringVar(&genSecurebootUKICmdFlags.commonName, "common-name", "Test UKI Signing Key", "common name for the certificate")
	genSecurebootCmd.AddCommand(genSecurebootUKICmd)

	genSecurebootCmd.AddCommand(genSecurebootPCRCmd)

	genSecurebootDatabaseCmd.Flags().StringVar(
		&genSecurebootDatabaseCmdFlags.enrolledCertificatePath, "enrolled-certificate", "_out/uki-signing-cert.pem", "path to the certificate to enroll")
	genSecurebootDatabaseCmd.Flags().StringVar(
		&genSecurebootDatabaseCmdFlags.signingCertificatePath, "signing-certificate", "_out/uki-signing-cert.pem", "path to the certificate used to sign the database")
	genSecurebootDatabaseCmd.Flags().StringVar(
		&genSecurebootDatabaseCmdFlags.signingKeyPath, "signing-key", "_out/uki-signing-key.pem", "path to the key used to sign the database")
	genSecurebootCmd.AddCommand(genSecurebootDatabaseCmd)
}
