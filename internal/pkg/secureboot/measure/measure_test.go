// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package measure_test

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/internal/pkg/secureboot/measure"
)

const (
	// ExpectedSignatureHex is output of `go test go test -v ./...` when systemd-measure binary is available.
	ExpectedSignatureHex = "e5fbb57a24951ad4c1621c7b1aa3071d0220d71cb8b498ff7f68ef431d70ee82ab12e6355259253366c839e2ec3dbb92caedb3398f5ceb6aa973666317d4a7f7"
)

func TestMeasureMatchesExpectedOutput(t *testing.T) {
	expectedSignatureHex := ExpectedSignatureHex

	if _, err := exec.LookPath("systemd-measure"); err == nil {
		expectedSignatureHex = getSignatureUsingSDMeasure(t)
	}

	tmpDir := t.TempDir()

	sectionsData := measure.SectionsData{}

	// create temporary files with the ordered section name and data as the section name
	for _, section := range secureboot.OrderedSections() {
		sectionFile := filepath.Join(tmpDir, string(section))

		if err := os.WriteFile(sectionFile, []byte(section), 0o644); err != nil {
			t.Fatal(err)
		}

		sectionsData[section] = sectionFile
	}

	pcrData, err := measure.GenerateSignedPCR(sectionsData, "testdata/pcr-signing-key.pem")
	if err != nil {
		t.Fatal(err)
	}

	pcrDataJSON, err := json.Marshal(&pcrData)
	if err != nil {
		t.Fatal(err)
	}

	pcrDataJSONHash := sha512.Sum512(pcrDataJSON)

	if hex.EncodeToString(pcrDataJSONHash[:]) != expectedSignatureHex {
		t.Fatalf("expected: %v, got: %v", expectedSignatureHex, hex.EncodeToString(pcrDataJSONHash[:]))
	}
}

func getSignatureUsingSDMeasure(t *testing.T) string {
	tmpDir := t.TempDir()

	sdMeasureArgs := make([]string, len(secureboot.OrderedSections()))

	// create temporary files with the ordered section name and data as the section name
	for i, section := range secureboot.OrderedSections() {
		sectionFile := filepath.Join(tmpDir, string(section))

		if err := os.WriteFile(sectionFile, []byte(section), 0o644); err != nil {
			t.Error(err)
		}

		sdMeasureArgs[i] = fmt.Sprintf("--%s=%s", strings.TrimPrefix(string(section), "."), sectionFile)
	}

	var signature bytes.Buffer

	sdCmd := exec.Command(
		"systemd-measure",
		append([]string{
			"sign",
			"--private-key",
			"testdata/pcr-signing-key.pem",
			"--phase=enter-initrd:leave-initrd:enter-machined",
			"--json=short",
		},
			sdMeasureArgs...,
		)...)

	sdCmd.Stdout = &signature

	if err := sdCmd.Run(); err != nil {
		t.Error(err)
	}

	s := bytes.TrimSpace(signature.Bytes())

	signatureHash := sha512.Sum512(s)

	hexEncoded := hex.EncodeToString(signatureHash[:])

	t.Log(hexEncoded)

	return hexEncoded
}
