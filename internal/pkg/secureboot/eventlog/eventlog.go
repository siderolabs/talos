// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package eventlog provides access to the TCG event log measured by the platform firmware.
package eventlog

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/google/go-eventlog/extract"
	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"

	"github.com/siderolabs/talos/internal/pkg/secureboot/tpm2"
	"github.com/siderolabs/talos/internal/pkg/tpm"
)

// Path is the securityfs location of the TCG2 event log measured by the platform firmware.
//
// The file is exposed by the TPM driver, so it is only present when the machine has a TPM
// and the firmware handed a log over to it.
const Path = "/sys/kernel/security/tpm0/binary_bios_measurements"

// SecureBootPCR is the PCR the firmware measures the SecureBoot policy into, along with
// every db entry it used to authorize an image it loaded.
const SecureBootPCR = 7

// ParseSecureBootState parses the SecureBoot state out of the PCR 7 events of rawLog.
//
// The events are replayed against pcr7, which must be the SHA-256 PCR 7 value read back from
// the TPM, so that a log which does not describe what was actually measured is rejected
// rather than believed.
func ParseSecureBootState(rawLog, pcr7 []byte) (*extract.SecurebootState, error) {
	eventLog, err := tcg.ParseEventLog(rawLog, tcg.ParseOpts{AllowPadding: true})
	if err != nil {
		return nil, fmt.Errorf("failed to parse event log: %w", err)
	}

	// only PCR 7 is replayed: everything this package cares about is measured there, and
	// replaying a PCR requires its current value, which would mean reading them all.
	events, err := eventLog.Verify([]register.MR{
		register.PCR{
			Index:     SecureBootPCR,
			Digest:    pcr7,
			DigestAlg: crypto.SHA256,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to replay PCR %d: %w", SecureBootPCR, err)
	}

	state, err := extract.ParseSecurebootState(events, extract.TPMRegisterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SecureBoot state: %w", err)
	}

	return state, nil
}

// SecureBootAuthorities returns the certificates from `db` which the firmware used to
// authorize the images loaded in this boot.
//
// Only the authorities recorded after the PCR 7 separator are returned: those are the ones
// which validated the boot loader and the OS image, whereas the ones before it belong to
// firmware drivers and option ROMs.
//
// The event log is measured by the firmware and is only present when the machine has a TPM,
// so a machine without one yields [os.ErrNotExist].
func SecureBootAuthorities() ([]x509.Certificate, error) {
	rawLog, err := os.ReadFile(Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read event log: %w", err)
	}

	pcr7, err := readPCR7()
	if err != nil {
		return nil, err
	}

	state, err := ParseSecureBootState(rawLog, pcr7)
	if err != nil {
		return nil, err
	}

	return state.PostSeparatorAuthority, nil
}

// readPCR7 reads the SHA-256 PCR 7 value from the TPM.
func readPCR7() ([]byte, error) {
	t, err := tpm.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open TPM device: %w", err)
	}

	defer t.Close() //nolint:errcheck

	pcr7, err := tpm2.ReadPCR(t, SecureBootPCR)
	if err != nil {
		return nil, fmt.Errorf("failed to read PCR %d: %w", SecureBootPCR, err)
	}

	return pcr7, nil
}
