// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cli

import (
	_ "embed"
	"os"
	"path/filepath"
)

var (
	//go:embed "testdata/pki/ca.crt"
	pkiCACrt []byte
	//go:embed "testdata/pki/ca.key"
	pkiCAKey []byte
	//go:embed "testdata/pki/front-proxy-ca.crt"
	pkiFrontProxyCACrt []byte
	//go:embed "testdata/pki/front-proxy-ca.key"
	pkiFrontProxyCAKey []byte
	//go:embed "testdata/pki/sa.key"
	pkiSAKey []byte
	//go:embed "testdata/pki/etcd/ca.crt"
	pkiEtcdCACrt []byte
	//go:embed "testdata/pki/etcd/ca.key"
	pkiEtcdCAKey []byte
)

func writeKubernetesPKIFiles(dir string) error {
	if err := os.Mkdir(dir, 0o777); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), pkiCACrt, 0o777); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, "ca.key"), pkiCAKey, 0o777); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, "front-proxy-ca.crt"), pkiFrontProxyCACrt, 0o777); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, "front-proxy-ca.key"), pkiFrontProxyCAKey, 0o777); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, "sa.key"), pkiSAKey, 0o777); err != nil {
		return err
	}

	etcdDir := filepath.Join(dir, "etcd")
	if err := os.Mkdir(etcdDir, 0o777); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(etcdDir, "ca.crt"), pkiEtcdCACrt, 0o777); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(etcdDir, "ca.key"), pkiEtcdCAKey, 0o777)
}
