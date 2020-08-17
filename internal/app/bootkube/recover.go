// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/kubernetes-sigs/bootkube/pkg/recovery"
	"go.etcd.io/etcd/clientv3"
	k8saes "k8s.io/apiserver/pkg/storage/value/encrypt/aes"

	"github.com/talos-systems/talos/internal/pkg/etcd"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

//nolint: gocyclo
func recoverAssets(config config.Provider) error {
	// Ensure assets directory does not exist / is left over from a failed install
	if err := os.RemoveAll(constants.AssetsDirectory); err != nil {
		// Ignore if the directory does not exist
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	var (
		backend recovery.Backend
		err     error
	)

	switch *recoverSource {
	case machineapi.RecoverRequest_ETCD.String():
		var client *clientv3.Client

		client, err = etcd.NewClient([]string{"127.0.0.1:2379"})
		if err != nil {
			return err
		}

		var transform recovery.TransformerFromStorage

		transform, err = aesTransformer(config.Cluster())
		if err != nil {
			return err
		}

		backend = recovery.NewEtcdBackendWithTransformer(client, "/registry", transform)
	case machineapi.RecoverRequest_APISERVER.String():
		backend, err = recovery.NewAPIServerBackend(constants.RecoveryKubeconfig)
		if err != nil {
			return err
		}
	}

	as, err := recovery.Recover(context.Background(), backend, constants.RecoveryKubeconfig)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(constants.AssetsDirectory, 0o600); err != nil {
		return err
	}

	if err = as.WriteFiles(constants.AssetsDirectory); err != nil {
		return fmt.Errorf("failed to write recovered assets: %w", err)
	}

	return nil
}

func aesTransformer(clusterConfig config.ClusterConfig) (recovery.TransformerFromStorage, error) {
	key, err := base64.StdEncoding.DecodeString(clusterConfig.AESCBCEncryptionSecret())
	if err != nil {
		return nil, err
	}

	cipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	transformer := k8saes.NewCBCTransformer(cipher)

	return func(value []byte) ([]byte, error) {
		const (
			aesCBCTransformerPrefixV1 = "k8s:enc:aescbc:v1:"
			aesCBCKeyName             = "key1:"

			aesCBCPrefix = aesCBCTransformerPrefixV1 + aesCBCKeyName
		)

		if !bytes.HasPrefix(value, []byte(aesCBCPrefix)) {
			return value, nil
		}

		value = value[len(aesCBCPrefix):]

		value, _, e := transformer.TransformFromStorage(value, nil)

		return value, e
	}, nil
}
