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

	_ "embed"

	"github.com/siderolabs/ukify/constants"
	"github.com/siderolabs/ukify/measure"
)

const (
	// ExpectedSignatureHex is output of `go test go test -v ./...` when systemd-measure binary is available
	ExpectedSignatureHex = "12e432978d18c9f720b3fb922cab180ca025ecd5f918966d1f878ae93f1eedbc6b20885d5a9f1c4ffdd4bf2dc3c25dc1097b6c5109d9c9a90128eff20056ace7"
)

//go:embed testdata/pcr-signing-key.pem
var pcrSigningKeyPEM []byte

func TestMeasureMatchesExpectedOutput(t *testing.T) {
	expectedSignatureHex := ExpectedSignatureHex

	if _, err := exec.LookPath("systemd-measure"); err == nil {
		expectedSignatureHex = getSignatureUsingSDMeasure(t)
	}

	tmpDir := t.TempDir()

	sectionsData := measure.SectionsData{}

	// create temporary files with the ordered section name and data as the section name
	for _, section := range constants.OrderedSections() {
		sectionFile := filepath.Join(tmpDir, string(section))

		if err := os.WriteFile(sectionFile, []byte(section), 0o644); err != nil {
			t.Fatal(err)
		}

		sectionsData[section] = sectionFile
	}

	signingKey := filepath.Join(tmpDir, "pcr-signing-key.pem")

	if err := os.WriteFile(signingKey, pcrSigningKeyPEM, 0o644); err != nil {
		t.Fatal(err)
	}

	pcrData, err := measure.GenerateSignedPCR(sectionsData, signingKey)
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
	tmpDir, err := os.MkdirTemp("", "measure-testdata-gen")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(tmpDir)

	sectionsData := measure.SectionsData{}
	sdMeasureArgs := make([]string, len(constants.OrderedSections()))

	// create temporary files with the ordered section name and data as the section name
	for i, section := range constants.OrderedSections() {
		sectionFile := filepath.Join(tmpDir, string(section))

		if err := os.WriteFile(sectionFile, []byte(section), 0o644); err != nil {
			panic(err)
		}

		sectionsData[section] = sectionFile
		sdMeasureArgs[i] = fmt.Sprintf("--%s=%s", strings.TrimPrefix(string(section), "."), sectionFile)
	}

	signingKey := filepath.Join(tmpDir, "pcr-signing-key.pem")

	if err := os.WriteFile(signingKey, pcrSigningKeyPEM, 0o644); err != nil {
		panic(err)
	}

	var signature bytes.Buffer

	sdCmd := exec.Command(
		"systemd-measure",
		append([]string{
			"sign",
			"--private-key",
			signingKey,
			"--json=short",
		},
			sdMeasureArgs...,
		)...)

	sdCmd.Stdout = &signature

	if err := sdCmd.Run(); err != nil {
		panic(err)
	}

	s := bytes.TrimSpace(signature.Bytes())

	signatureHash := sha512.Sum512(s)

	hexEncoded := hex.EncodeToString(signatureHash[:])

	t.Log(hexEncoded)

	return hexEncoded
}
