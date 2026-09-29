// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type KubeletKubeconfigSuite struct {
	ctest.DefaultSuite

	kubeconfigPath string

	// logs captures controller errors, so that tests can assert that the controller rides
	// over transient on-disk state instead of failing and being restarted
	logs *observer.ObservedLogs
}

func TestKubeletKubeconfigSuite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig-kubelet")

	observerCore, logs := observer.New(zapcore.ErrorLevel)

	s := &KubeletKubeconfigSuite{
		kubeconfigPath: path,
		logs:           logs,
	}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		Logger:  zap.New(observerCore),
		AfterSetup: func(ds *ctest.DefaultSuite) {
			// Reset filesystem and log state so tests don't leak into each other.
			logs.TakeAll()

			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				ds.Require().NoError(err)
			}

			ds.Require().NoError(ds.Runtime().RegisterController(&k8sctrl.KubeletKubeconfigController{
				Path: path,
			}))
		},
	}

	suite.Run(t, s)
}

func hashOf(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

func (suite *KubeletKubeconfigSuite) writeKubeconfig(data []byte) {
	suite.T().Helper()

	suite.Require().NoError(os.WriteFile(suite.kubeconfigPath, data, 0o600))
}

// completeKubeconfig builds a kubeconfig carrying everything the controller requires to
// consider it fully written; the server URL is a handy way to get a different one.
func completeKubeconfig(server string) []byte {
	return fmt.Appendf(nil,
		`apiVersion: v1
kind: Config
clusters:
- name: default-cluster
  cluster:
    server: %s
contexts:
- name: default-context
  context:
    cluster: default-cluster
    user: default-auth
current-context: default-context
users:
- name: default-auth
  user:
    token: bootstrap-token
`, server)
}

func (suite *KubeletKubeconfigSuite) TestMissingFileNoResource() {
	ctest.AssertNoResource[*k8s.KubeletKubeconfig](suite, k8s.KubeletKubeconfigID)
}

func (suite *KubeletKubeconfigSuite) TestCreateUpdateDelete() {
	initial := completeKubeconfig("https://localhost:6443")

	suite.writeKubeconfig(initial)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(hashOf(initial), res.TypedSpec().Hash)
		},
	)

	updated := completeKubeconfig("https://localhost:7445")

	suite.writeKubeconfig(updated)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(hashOf(updated), res.TypedSpec().Hash)
		},
	)

	suite.Require().NoError(os.Remove(suite.kubeconfigPath))

	ctest.AssertNoResource[*k8s.KubeletKubeconfig](suite, k8s.KubeletKubeconfigID)
}

// newCertificatePEM generates a self-signed certificate and its key in PEM format.
func newCertificatePEM(t *testing.T) (crt, key []byte) {
	t.Helper()

	ca, err := x509.NewSelfSignedCertificateAuthority()
	require.NoError(t, err)

	return ca.CrtPEM, ca.KeyPEM
}

func (suite *KubeletKubeconfigSuite) writeKubeconfigWithClientCert(certPath string) []byte {
	suite.T().Helper()

	kubeconfig := kubeconfigWithClientCert(certPath)

	suite.writeKubeconfig(kubeconfig)

	return kubeconfig
}

func kubeconfigWithClientCert(certPath string) []byte {
	return fmt.Appendf(nil,
		`apiVersion: v1
kind: Config
clusters:
- name: default-cluster
  cluster:
    server: https://localhost:6443
contexts:
- name: default-context
  context:
    cluster: default-cluster
    user: default-auth
current-context: default-context
users:
- name: default-auth
  user:
    client-certificate: %s
    client-key: %s
`, certPath, certPath)
}

// hashWithoutCert is the hash the controller publishes for a kubeconfig whose referenced
// certificate file is missing: only the kubeconfig and the path it points at are digested.
func hashWithoutCert(kubeconfig []byte, certPath string) string {
	sum := sha256.New()

	sum.Write(kubeconfig)
	sum.Write([]byte(certPath))

	return hex.EncodeToString(sum.Sum(nil))
}

// TestClientCertificateRotation verifies that the hash follows the certificate the
// kubeconfig points at: kubelet rotates it behind the `kubelet-client-current.pem`
// symlink without ever rewriting the kubeconfig itself.
func (suite *KubeletKubeconfigSuite) TestClientCertificateRotation() {
	pkiDir := filepath.Join(filepath.Dir(suite.kubeconfigPath), "pki")
	suite.Require().NoError(os.MkdirAll(pkiDir, 0o700))

	// kubelet keeps the client key in the very same file as the client certificate
	certPath := filepath.Join(pkiDir, "kubelet-client-current.pem")

	crt, key := newCertificatePEM(suite.T())
	suite.Require().NoError(os.WriteFile(certPath, slices.Concat(key, crt), 0o600))

	suite.writeKubeconfigWithClientCert(certPath)

	var initialHash string

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.NotEmpty(res.TypedSpec().Hash)

			initialHash = res.TypedSpec().Hash
		},
	)

	// rotate the certificate, keeping the kubeconfig untouched
	rotatedCrt, rotatedKey := newCertificatePEM(suite.T())
	suite.Require().NoError(os.WriteFile(certPath, slices.Concat(rotatedKey, rotatedCrt), 0o600))

	var (
		rotatedHash    string
		rotatedVersion resource.Version
	)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.NotEqual(initialHash, res.TypedSpec().Hash)

			rotatedHash = res.TypedSpec().Hash
			rotatedVersion = res.Metadata().Version()
		},
	)

	// replacing the key alone should not be visible in the hash: private key material is
	// deliberately left out of the digest
	_, otherKey := newCertificatePEM(suite.T())
	suite.Require().NoError(os.WriteFile(certPath, slices.Concat(otherKey, rotatedCrt), 0o600))

	// give the fsnotify-driven reconcile a chance to run: an unchanged hash is a no-op
	// write, so the resource version stays put as well
	time.Sleep(time.Second)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(rotatedHash, res.TypedSpec().Hash)
			assert.Equal(rotatedVersion.String(), res.Metadata().Version().String())
		},
	)
}

// TestPKIDirectoryRecreated verifies that the watch on kubelet's PKI directory survives the
// directory being removed and recreated: Talos wipes it when the client certificate no
// longer verifies against the accepted CAs, and the kernel drops the watch along with it.
func (suite *KubeletKubeconfigSuite) TestPKIDirectoryRecreated() {
	pkiDir := filepath.Join(filepath.Dir(suite.kubeconfigPath), "pki-recreated")
	suite.Require().NoError(os.MkdirAll(pkiDir, 0o700))

	certPath := filepath.Join(pkiDir, "kubelet-client-current.pem")

	crt, key := newCertificatePEM(suite.T())
	suite.Require().NoError(os.WriteFile(certPath, slices.Concat(key, crt), 0o600))

	kubeconfig := suite.writeKubeconfigWithClientCert(certPath)

	withoutCert := hashWithoutCert(kubeconfig, certPath)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.NotEqual(withoutCert, res.TypedSpec().Hash)
		},
	)

	suite.Require().NoError(os.RemoveAll(pkiDir))
	suite.Require().NoError(os.MkdirAll(pkiDir, 0o700))

	// Rewrite the kubeconfig to get a reconcile in now that the directory is back: in
	// production any reconcile does, and the poll tick guarantees one. Waiting for the
	// certificate-less hash to be published pins down that the reconcile has happened, so
	// that the watch, if it is going to be re-established at all, is in place by now.
	suite.writeKubeconfigWithClientCert(certPath)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(withoutCert, res.TypedSpec().Hash)
		},
	)

	// The kubeconfig is untouched from here on, so this is only picked up if the recreated
	// directory got watched anew: nothing else reports a write to a file inside it.
	newCrt, newKey := newCertificatePEM(suite.T())
	suite.Require().NoError(os.WriteFile(certPath, slices.Concat(newKey, newCrt), 0o600))

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.NotEqual(withoutCert, res.TypedSpec().Hash)
		},
	)
}

// TestMalformedKubeconfigKeepsLastHash verifies that a partially written kubeconfig
// doesn't get hashed: neither kubelet nor Talos rewrite the file atomically, and hashing a
// torn file would make every consumer rebuild its Kubernetes client for nothing.
func (suite *KubeletKubeconfigSuite) TestMalformedKubeconfigKeepsLastHash() {
	initial := completeKubeconfig("https://localhost:6443")

	suite.writeKubeconfig(initial)

	var version resource.Version

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(hashOf(initial), res.TypedSpec().Hash)

			version = res.Metadata().Version()
		},
	)

	suite.writeKubeconfig([]byte("apiVersion: v1\nkind: Config\nclusters:\n- name: \"unterminated\n"))

	// give the fsnotify-driven reconcile a chance to run: the hash published so far is kept,
	// so the resource is neither updated nor destroyed
	time.Sleep(time.Second)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(hashOf(initial), res.TypedSpec().Hash)
			assert.Equal(version.String(), res.Metadata().Version().String())
		},
	)

	suite.assertControllerDidNotFail()

	// once the file is written completely, the hash follows it again
	updated := completeKubeconfig("https://localhost:7445")

	suite.writeKubeconfig(updated)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(hashOf(updated), res.TypedSpec().Hash)
		},
	)
}

// TestIncompleteKubeconfigKeepsLastHash verifies that a kubeconfig which parses but carries
// no credentials doesn't get hashed either: the file is rewritten in place with `O_TRUNC`,
// and that truncation is itself an fsnotify event, so a read is quite likely to catch the
// file at zero length — for which [clientcmd.Load] reports an empty config and no error.
func (suite *KubeletKubeconfigSuite) TestIncompleteKubeconfigKeepsLastHash() {
	initial := completeKubeconfig("https://localhost:6443")

	suite.writeKubeconfig(initial)

	var version resource.Version

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(hashOf(initial), res.TypedSpec().Hash)

			version = res.Metadata().Version()
		},
	)

	for _, incomplete := range [][]byte{
		// the file as `os.WriteFile` leaves it right after truncating
		nil,
		// a prefix of the kubeconfig which is valid YAML on its own
		[]byte("apiVersion: v1\nkind: Config\nclusters: []\n"),
	} {
		suite.writeKubeconfig(incomplete)

		// give the fsnotify-driven reconcile a chance to run: the hash published so far is
		// kept, so the resource is neither updated nor destroyed
		time.Sleep(time.Second)

		ctest.AssertResource(
			suite,
			k8s.KubeletKubeconfigID,
			func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
				assert.Equal(hashOf(initial), res.TypedSpec().Hash)
				assert.Equal(version.String(), res.Metadata().Version().String())
			},
		)
	}

	suite.assertControllerDidNotFail()
}

// TestMalformedKubeconfigNoResource verifies that a kubeconfig which has never been
// readable doesn't produce a resource at all: consumers should keep waiting instead of
// rebuilding their clients around a half-written file.
func (suite *KubeletKubeconfigSuite) TestMalformedKubeconfigNoResource() {
	suite.writeKubeconfig([]byte("apiVersion: v1\nkind: Config\nclusters:\n- name: \"unterminated\n"))

	ctest.AssertNoResource[*k8s.KubeletKubeconfig](suite, k8s.KubeletKubeconfigID)

	suite.assertControllerDidNotFail()
}

// assertControllerDidNotFail asserts that the controller kept running rather than
// returning an error and being restarted by the runtime.
func (suite *KubeletKubeconfigSuite) assertControllerDidNotFail() {
	suite.T().Helper()

	suite.Assert().Empty(suite.logs.FilterMessage("controller failed").All())
}
