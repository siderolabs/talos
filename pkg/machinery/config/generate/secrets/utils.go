// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/siderolabs/crypto/x509"
)

func randBytes(size int) ([]byte, error) {
	buf := make([]byte, size)

	n, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read from random generator: %w", err)
	}

	if n != size {
		return nil, fmt.Errorf("failed to generate sufficient number of random bytes (%d != %d)", n, size)
	}

	return buf, nil
}

func validatePEMEncodedCertificateAndKey(certs *x509.PEMEncodedCertificateAndKey) error {
	_, err := certs.GetKey()
	if err != nil {
		return err
	}

	_, err = certs.GetCert()

	return err
}

// randBootstrapTokenString returns a random string consisting of the characters in
// validBootstrapTokenChars, with the length customized by the parameter.
func randBootstrapTokenString(length int) (string, error) {
	// validBootstrapTokenChars defines the characters a bootstrap token can consist of
	const validBootstrapTokenChars = "0123456789abcdefghijklmnopqrstuvwxyz"

	// len("0123456789abcdefghijklmnopqrstuvwxyz") = 36 which doesn't evenly divide
	// the possible values of a byte: 256 mod 36 = 4. Discard any random bytes we
	// read that are >= 252 so the bytes we evenly divide the character set.
	const maxByteValue = 252

	var (
		b     byte
		err   error
		token = make([]byte, length)
	)

	reader := bufio.NewReaderSize(rand.Reader, length*2)

	for i := range token {
		for {
			if b, err = reader.ReadByte(); err != nil {
				return "", err
			}

			if b < maxByteValue {
				break
			}
		}

		token[i] = validBootstrapTokenChars[int(b)%len(validBootstrapTokenChars)]
	}

	return string(token), err
}

// genToken will generate a token of the format abc.123 (like kubeadm/trustd), where the length of the first string (before the dot)
// and length of the second string (after dot) are specified as inputs.
func genToken(lenFirst, lenSecond int) (string, error) {
	var err error

	tokenTemp := make([]string, 2)

	tokenTemp[0], err = randBootstrapTokenString(lenFirst)
	if err != nil {
		return "", err
	}

	tokenTemp[1], err = randBootstrapTokenString(lenSecond)
	if err != nil {
		return "", err
	}

	return tokenTemp[0] + "." + tokenTemp[1], nil
}
