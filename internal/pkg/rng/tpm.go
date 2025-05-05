// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rng

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/go-tpm/tpm2"

	"github.com/siderolabs/talos/internal/pkg/tpm"
)

// TPMSeed seeds the random entropy pool from the TPM.
//
//nolint:gocyclo
func TPMSeed() error {
	t, err := tpm.Open()
	if err != nil {
		// if the TPM is not available we can skip seeding random pool
		if os.IsNotExist(err) {
			log.Printf("TPM device is not available")

			return nil
		}

		return fmt.Errorf("error opening TPM device: %w", err)
	}

	defer t.Close() //nolint:errcheck

	// now we need to check if the TPM is a 2.0 device
	// we can do this by checking the manufacturer,
	// if it fails, we can skip trying to seed the pool
	caps, err := tpm2.GetCapability{
		Capability:    tpm2.TPMCapTPMProperties,
		Property:      uint32(tpm2.TPMPTManufacturer),
		PropertyCount: 1,
	}.Execute(t)
	if err != nil {
		log.Printf("TPM device is not a TPM 2.0, skipping seeding entropy pool from TPM")

		return nil
	}

	props, err := caps.CapabilityData.Data.TPMProperties()
	if err != nil {
		return fmt.Errorf("error getting properties: %w", err)
	}

	log.Printf("TPM manufacturer ID: %08x", props.TPMProperty[0].Value)

	poolSize, err := GetPoolSize()
	if err != nil {
		return fmt.Errorf("error getting pool size: %w", err)
	}

	remaining := poolSize
	start := time.Now()

	for remaining > 0 {
		chunk := 32 // default to small chunk (size of AES key)
		if remaining < chunk {
			chunk = remaining
		}

		cmd := tpm2.GetRandom{
			BytesRequested: uint16(chunk),
		}

		resp, err := cmd.Execute(t)
		if err != nil {
			return fmt.Errorf("error getting random data from the TPM: %w", err)
		}

		if len(resp.RandomBytes.Buffer) == 0 {
			return fmt.Errorf("received zero random bytes from the TPM: %w", err)
		}

		if err = WriteEntropy(resp.RandomBytes.Buffer); err != nil {
			return fmt.Errorf("error writing random pool entropy: %w", err)
		}

		remaining -= len(resp.RandomBytes.Buffer)
	}

	log.Printf("seeded random pool with %d bytes from the TPM in %s", poolSize, time.Since(start))

	return nil
}
