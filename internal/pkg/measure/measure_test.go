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

	"github.com/siderolabs/talos/internal/pkg/measure"
	"github.com/siderolabs/talos/internal/pkg/measure/internal/pcr"
)

const (
	// ExpectedSignatureJSON is pre-calculated signature.
	//nolint:lll
	ExpectedSignatureJSON = `{"sha256":[{"pcrs":[11],"pkfp":"58f58f625bd8a8b6681e4b40688cf99b26419b6b2c5f6e14a2c7c67a3b0b1620","pol":"c0c4f61f8ac39267a7638cde8029b82a0ccd378b2acbfdffb77ee1f0f9d464ec","sig":"zyyb7nNjQP6GyM1Y2TtCo2FbSDSLMYzDIw5sgsm3vWDgWK6bZnItxixA1J9pF8ccqW09VH5kQU3a5xFl1ZsNmZwSUtK1wr9jwITSW4V+G2508gt1X3t0Yq9SXfXo8JNhKhayjtcfLKEIj3NvaCEgEMwcJJWUM/fooWeoXOdlx3JfTLcL1Pog6Hy7o4nDGKGMHsxUf1RNzf2Ro+Z4lXQ1334fqLeoC2ZbQbFyjRuGMin0/QvWZQ0k8FQ6ZooZR/SrJQrUu/ouzrzyEFIOBLfituBjw6fnT40ieZUh9N/bPqrX9jtKT3eBICYoEuY5V67bpF1Ygm+sDarBvR9MYVBY4DSm1iIOtABBUKVaACXxG4dFrEAUydXv62Yd9Kl86DqoBXrQRu+dcBKa1Stw/eGhzzoaXQo4XeutnIj3QOwtOHN1Z/L02gcZdlCcRboGAiTCWs5m642oSJa8jiAKWzpgwsJpSDkHRsmjWewQMBccgDy1j6DiHp4HakgeK2pePWdD/c8pu8unThKigKy/wsH+QMXpzNufkqAq2aWyuempFDn+OdVHGE5htZuSNIa66Vj0sd1guH13K9zxNUN8JguWnk21zInAN1IpbmyxlgRYCEpuE++E5Te4tqskCHUAU4D1iLS8a73SK5wXS7AnMXK7XhQ4FFvJCaOaQiJjC30LnEk="}],"sha384":[{"pcrs":[11],"pkfp":"58f58f625bd8a8b6681e4b40688cf99b26419b6b2c5f6e14a2c7c67a3b0b1620","pol":"e8f6311e36f9ee038df65255ae7c471d1d1f91ce742b98a152be72337f6c937d","sig":"SH9VntNIwbApIoTM9GksYp6fScBkjQ156rEscm9cR6hFcIy8oZeFCzaCxfl122UgNVzXMZDst4S3xF/BjwG1TiUE3SvzlvCwzvAkLVitHzD3Jj/oyy5vIfe7lhrPvSSb5ta3I5DsBE4693OlXDw/hXzoj93i/Cbztqf/N7G354tO2COx8CXu63Yz3DBijdvDN2lIA6AVv5AV+IC0koU8wuKsdOmbHoysDI/JtFi5f2qQP667qjOUJj2FgsLoH//ZvlKTZjIrGsQo9q6iQsxAEdBKdWub8mrexgalH4jzHfFyVxDDeV6+D77xFCAuaT1fVOcGKTncxAvxN2M+wg7XDEVnZm6Itmh+Qh8DKejogHTBVr1ALBzE12hSpvIlYHQ6bNrzORPX/+7eJ3nbnTfYEp9psyFbxV+21U7A7ArwDQLnhosbVFY94YSyQd13bfgWQll013HzcpiTuzg3+qU4DjPc0x764z614+K9kyJQLqhiE0Y82Qhelhu+dJa/DrmOPEXQ6Lt5YG5TPQSHkNq79vPF1JUC1OkMvnFwO10eU2zPbuVKN7mf9S2ERSGEB9oX7pYLf99gNDh+VCII1iP+Jzj7jcAdvt6VK1Cme89kzXWem66eLaDNYIibb18KJ0uArqBt8VBzsX5wUb9sQTo/f2DOPI2AvHrzWziXjUG8cWc="}],"sha512":[{"pcrs":[11],"pkfp":"58f58f625bd8a8b6681e4b40688cf99b26419b6b2c5f6e14a2c7c67a3b0b1620","pol":"207a8217158f9cc521dca50358a593dc61ed9308ef27da9df4ef64d634309160","sig":"NY659iroM9POX2a75p26i8JPXd+/Tsb45Wd/FEmjb1gopY4ygRumFNn/YuOebbhxHq67SMB30Fz5R1ktLJHgiCZotg5BqiUZWKF1zVlze928LTZqS4xxU5eUlZuzFkDyokwuxzvmc1+qFotkP1FMq8LSyDpBpUKby26EOP8cLYuE6PKc3LmhvjTmOUglICyv0JAoPRARDjZEN/f1O2T9HnACYjXZIVuKkBhMyYkuIgP+kGR1ChnHZTvcOkFByZcOUWtX6ChAP+OvDYQ1lbpomTyyKLmBvA935kJCCL1cDv3u5LCCaPfLm7zAVIyOrfELUpt8I3Oe/qIRX7CiP/TzN3IaNPOzU4+rm0vLcM56T5bEfUw4ikQILykQ202hUeH9Zv5Fw/qZI2nU4ToRamtLQd28xKfod9Uq2fbfORybUU5Ab2SbXpcwMu9PUsapFpjf9hrC1jf/G26TJ1mEzhhLPgJ1UlKyFX6ba0urhudR9dr3xLHlh3gw+T1NnZVram1aYBJnOU6lxHcoD+wmop3pqu1S0VOJX9YIGKzZ5jrzJ102cAI58nihASIkaWD7JqisFBCp2jX0ETu6bJLcufLv7ekT+asw2K1C8BxH5INiRkXu1HEPLYhYBceAgEFGROI26Awy08o8axsCt2zSAi2EafnCEt8mIo5E5RDGvRbRsWk="}]}`
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
	for _, section := range pcr.OrderedSections() {
		sectionFile := filepath.Join(tmpDir, section)

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

	sdMeasureArgs := make([]string, len(pcr.OrderedSections()))

	// create temporary files with the ordered section name and data as the section name
	for i, section := range pcr.OrderedSections() {
		sectionFile := filepath.Join(tmpDir, section)

		if err := os.WriteFile(sectionFile, []byte(section), 0o644); err != nil {
			t.Error(err)
		}

		sdMeasureArgs[i] = fmt.Sprintf("--%s=%s", strings.TrimPrefix(section, "."), sectionFile)
	}

	var (
		signature bytes.Buffer
		stderr    bytes.Buffer
	)

	sdCmd := exec.CommandContext(
		t.Context(),
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
	sdCmd.Stderr = &stderr

	t.Log("Running systemd-measure command:", sdCmd.String())

	if err := sdCmd.Run(); err != nil {
		t.Log("stderr:", stderr.String())
		t.Fatalf("systemd-measure failed: %v", err)
	}

	s := bytes.TrimSpace(signature.Bytes())

	return string(s)
}
