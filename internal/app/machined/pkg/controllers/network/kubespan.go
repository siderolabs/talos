// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"errors"
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	kubeSpanPrivateKeyMutex sync.Mutex

	kubeSpanPrivateKeyFile string
)

func init() {
	kubeSpanPrivateKeyFile = path.Join(constants.StateMountPoint, "kubespan.key")
}

func loadOrCreatePrivateKey() (string, error) {
	kubeSpanPrivateKeyMutex.Lock()
	defer kubeSpanPrivateKeyMutex.Unlock()

	_, err := os.Stat(kubeSpanPrivateKeyFile)
	if errors.Is(err, os.ErrNotExist) {
		return generatePrivateKeyFile(kubeSpanPrivateKeyFile)
	}

	key, err := os.ReadFile(kubeSpanPrivateKeyFile)
	if err != nil {
		return "", fmt.Errorf("failed to read private key from file %q: %w", kubeSpanPrivateKeyFile, err)
	}

	return string(key), nil
}

func generatePrivateKeyFile(fn string) (string, error) {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate private key: %w", err)
	}

	keyString := key.String()

	if err = os.WriteFile(fn, []byte(keyString), 0o400); err != nil {
		return "", fmt.Errorf("failed to write private key to file %q: %w", fn, err)
	}

	return keyString, nil
}
