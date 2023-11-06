// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package measure_test

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/internal/pkg/secureboot/measure"
)

const (
	// ExpectedSignatureJSON is pre-calculated signature.
	//nolint:lll
	ExpectedSignatureJSON = `{"sha256":[{"pcrs":[11],"pkfp":"58f58f625bd8a8b6681e4b40688cf99b26419b6b2c5f6e14a2c7c67a3b0b1620","pol":"88b9f03a2edc4960894b5fa460169282749aff60d0b491ca71a2bde937f63f28","sig":"JBOKtBN2YAvugIqpPK6aulzCOAscdFojCqkawL9xa43tmhCj2wnfbrskym1QLlozYyiD6/6HpEYB91J71vWtTQTNUiJP910+qvcPkREHUdiPdzBTEDXDklvB9ACXfzcEl1lEhqlYfjJ9nxTmapWjOOL/iV/GgamkAtEwT91HE0ITf6ESedSt0+0T9lft6W10i7nO6WPJfQ2WDAZ6tGZUJLpvf36ZPloxRKXCCGkhtI1FYetT2ZJHgamrm6rGWa+IkOwFBydBjdSg20HDqNPFqzPlXLagx1SxXIPlH9Et+ebiFBRhevgD2GDL52wY3XS88G0AuYAlFrkrmx72wNZ7v/f16srXGOUSLwIzVE10z88nS+TYEUugKtIwrj1h165thMwcQONPSj/tL2OKcmM6jpAtIpApZUYOKip+sS5cKBloqhNbVuu6lzexi7THYWpn+Bzlj5zu7VO6yDLb8FaCxgTdoCiR8/q5yzXa8+LBJX1wfdvktDYGdmO0h/nGQYjOfKCSV2WCZLxIiNPedWkUX7hMIggTaaYiaxwYJHIZNKS0N3H1FGkSSFnYNJFHyW0S7BSi8dF9l0VlgXh6HrwSbMyv1avNQiL/pP7CwyA/dyYdsON6neWFRoYfaKRZdrQRKvH4VhjdB4CGLgxB7Pom8BQc8Vi68sAsKLKsNXY8GSs="}],"sha384":[{"pcrs":[11],"pkfp":"58f58f625bd8a8b6681e4b40688cf99b26419b6b2c5f6e14a2c7c67a3b0b1620","pol":"532380d1bfae365cca029f2e4a57601d528a346bfd9feae1e1f3f994260fb56d","sig":"qKilaBxg/goIw3TXfd+kIMqfA1EOcWngv+iH/xUliHLyxzTVHxx4vF4+wVJUNmS7vj10ffU876cySqyV9S9EFSPn3EEHLKza5n4skf++IZj3UvBXRwXTKKnE+iLDvj3r8maEorOD5J9rlv7nZZ95HESNxy0J2lXa0ik+vGKaKqySkyqFF2xM22M9Ko3iGr2bTSK8faGcorORNe/Hjr3GeJWm66Ea1EJvlIR9HjwDdeY+RMF/WEtSYMVkRM6ZYp/j2epTOTWsjUqYECxcWxjydrXm+Sf1ZDrHGURBDVHxpfTDWfrdpQ33tyappjwSnwH2zVLqThG6DEUqXmvn0/uYnSjLlKjRVuJyMotrcHSymot2oPQ/olfReOUAOkQR2TqiFEN5NeYAEghMDzwyYv1Rr5ahOJPcPMpEPnztUem4+kaAlOW4aAhuKcrgoiNle4G68CE0d3SEL3JvF3pyg9zSqai59vN8dd7Gw3vDYETPsOFWL7UJKb0DCCDCEFBN9kSvRScjjsz18NfJ0C5df7VhJC+P0WukxAPxQPvaK99HnD4JPpa9P0lEqubb+bvzZ+YuR6zblLVEt+4AISt/4Td0F8ylyMaz4+Rj+zm+VCCrNz3zqIjbLMQ/k1v6rRd/Yjre8DyQjSh7vD2eZr0BopUmimv5gGXvCranbHaq2jSXI5w="}],"sha512":[{"pcrs":[11],"pkfp":"58f58f625bd8a8b6681e4b40688cf99b26419b6b2c5f6e14a2c7c67a3b0b1620","pol":"2ea15476710c3d5e2ac70b6cee68a6a7619c91f69cf2d1387ad567f3fb5076fe","sig":"2SuhxgPM5hjb//gqud9Lomsjss/UTHz8B6wpcyUQHCroGHxd7OoDfzUd3nuXOf7BRbzDRiwo/sX4cR1CDDer9R4ut6QpQ6BHjPdcPxnH06BZvNTDubmU3gWJA49vZDD3IBnEWp/cMyGtVVbYuidy790EP+D9nBs247Cx4B1cepVcO3T5UHrrXst7oOt7OvTC+j3tPrGedhuEmRI0g+ms3VBeX4fcTFkqsjJmEI8kEz+otc388o9m/Edd5jxj42nf+Fu3RKH6GaLicQOr8Eat2RfMwfnv6ZxLhlwolwb/56DGgOJ0yuNEUuLvNru60KBXPjpTurUVSvYqL7qxrz7LqOOm+TMOgUjDV481GFwma6RvVOaAxlbPKwFwJSeMGqyhZBCOXRUraFfD0vJNwhB+BCaEmgXpFLjhX7IsUwieLq73vy9zRp4x+/fyhj2rnAfNku04JovBy7PYI/1nfjmdBoSvCyPmLCu5pznU61aMgpm1O1VgNdsSEqCu/pej724WGJ8tMjlhPHDgzsPnKu68m/0K+Kh5oYlP4uy04NtHXsif37bhdLmg09NcPmI7UEl+h1CGLNT2jf9tklvtDqVfzLyXnnbIoJFasO542Xx9RwlvAWYgs95jZIMYSpE+8o+Eg+rMyW7H/ej2M87HfQltv+R2Jb5Fm99D5kmrdnoH1vc="}]}`
)

type rsaWrapper struct {
	*rsa.PrivateKey
}

func (w rsaWrapper) PublicRSAKey() *rsa.PublicKey {
	return &w.PrivateKey.PublicKey
}

func loadRSAKey(path string) (measure.RSAKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// convert private key to rsa.PrivateKey
	rsaPrivateKeyBlock, _ := pem.Decode(keyData)
	if rsaPrivateKeyBlock == nil {
		return nil, err
	}

	rsaKey, err := x509.ParsePKCS1PrivateKey(rsaPrivateKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key failed: %v", err)
	}

	return rsaWrapper{rsaKey}, nil
}

func TestMeasureMatchesExpectedOutput(t *testing.T) {
	expectedSignatureHex := ExpectedSignatureJSON

	if _, err := exec.LookPath("systemd-measure"); err == nil {
		t.Log("systemd-measure binary found, using it to get expected signature")
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

	rsaKey, err := loadRSAKey("testdata/pcr-signing-key.pem")
	if err != nil {
		t.Fatal(err)
	}

	pcrData, err := measure.GenerateSignedPCR(sectionsData, rsaKey)
	if err != nil {
		t.Fatal(err)
	}

	pcrDataJSON, err := json.Marshal(&pcrData)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, expectedSignatureHex, string(pcrDataJSON))
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
			"--bank=sha256",
			"--bank=sha384",
			"--bank=sha512",
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

	return string(s)
}
