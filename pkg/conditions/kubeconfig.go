// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"k8s.io/client-go/tools/clientcmd"
)

type kubeconfig string

func (filename kubeconfig) Wait(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		_, err := os.Stat(string(filename))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		_, err = clientcmd.BuildConfigFromFlags("", string(filename))
		if err == nil {
			return nil
		}

		// TODO: we can't check for specific error here (looking for file not found for client key/cert):
		//       https://github.com/kubernetes/kubernetes/pull/105080

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (filename kubeconfig) String() string {
	return fmt.Sprintf("kubeconfig %q to be ready", string(filename))
}

// WaitForKubeconfigReady is a condition that will wait for the kubeconfig to be ready.
func WaitForKubeconfigReady(filename string) Condition {
	return kubeconfig(filename)
}
