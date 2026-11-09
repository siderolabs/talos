// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestRootSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RootSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(secretsctrl.NewRootEtcdController()))
			suite.Require().NoError(suite.Runtime().RegisterController(secretsctrl.NewRootKubernetesController()))
			suite.Require().NoError(suite.Runtime().RegisterController(secretsctrl.NewRootOSController()))
		},
	})
}

type RootSuite struct {
	ctest.DefaultSuite
}

func (suite *RootSuite) genConfig(controlplane bool, versionContract *talosconfig.VersionContract) talosconfig.Config {
	input, err := generate.NewInput(
		"test-cluster", "http://localhost:6443", "1.28.0",
		generate.WithVersionContract(versionContract),
	)
	suite.Require().NoError(err)

	var cfg talosconfig.Provider

	if controlplane {
		cfg, err = input.Config(machine.TypeControlPlane)
	} else {
		cfg, err = input.Config(machine.TypeWorker)
	}

	suite.Require().NoError(err)

	machineCfg := config.NewMachineConfig(cfg)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineCfg))

	return cfg
}

func (suite *RootSuite) TestReconcileControlPlane() {
	cfg := suite.genConfig(true, talosconfig.TalosVersionCurrent)

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.EtcdRootID},
		func(res *secrets.EtcdRoot, asrt *assert.Assertions) {
			asrt.NotNil(res.TypedSpec().EtcdCA)
			asrt.Equal(res.TypedSpec().EtcdCA, cfg.EtcdCAConfig().IssuingCA())
			asrt.Equal(
				[]*x509.PEMEncodedCertificate{
					{
						Crt: cfg.EtcdCAConfig().IssuingCA().Crt,
					},
				},
				res.TypedSpec().AcceptedCAs,
			)
		},
	)
	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.KubernetesRootID},
		func(res *secrets.KubernetesRoot, asrt *assert.Assertions) {
			asrt.Equal(res.TypedSpec().IssuingCA, cfg.K8sAPIServerCAConfig().IssuingCA())
			asrt.Equal(
				cfg.K8sAPIServerCAConfig().AcceptedCAs(),
				res.TypedSpec().AcceptedCAs,
			)
			asrt.Empty(res.TypedSpec().AESCBCEncryptionSecret)
			asrt.Empty(res.TypedSpec().SecretboxEncryptionSecret)
			asrt.NotEmpty(res.TypedSpec().EtcdEncryptionConfig)
		},
	)

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.OSRootID},
		func(res *secrets.OSRoot, asrt *assert.Assertions) {
			asrt.Equal(res.TypedSpec().IssuingCA, cfg.Machine().Security().IssuingCA())
			asrt.Equal(
				[]*x509.PEMEncodedCertificate{
					{
						Crt: cfg.Machine().Security().IssuingCA().Crt,
					},
				},
				res.TypedSpec().AcceptedCAs,
			)
		},
	)
}

func (suite *RootSuite) TestReconcileControlPlaneLegacyEtcd() {
	// Talos 1.14 generates etcd config in the legacy .cluster.etcd section
	cfg := suite.genConfig(true, talosconfig.TalosVersion1_14)

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.EtcdRootID},
		func(res *secrets.EtcdRoot, asrt *assert.Assertions) {
			asrt.NotNil(res.TypedSpec().EtcdCA)
			asrt.Equal(res.TypedSpec().EtcdCA, cfg.EtcdCAConfig().IssuingCA())
			asrt.Equal(
				[]*x509.PEMEncodedCertificate{
					{
						Crt: cfg.EtcdCAConfig().IssuingCA().Crt,
					},
				},
				res.TypedSpec().AcceptedCAs,
			)
		},
	)
}

func (suite *RootSuite) TestReconcileControlPlaneEtcdAcceptedCAs() {
	input, err := generate.NewInput(
		"test-cluster", "http://localhost:6443", "1.28.0",
		generate.WithVersionContract(talosconfig.TalosVersionCurrent),
	)
	suite.Require().NoError(err)

	cfg, err := input.Config(machine.TypeControlPlane)
	suite.Require().NoError(err)

	extraCA, err := x509.NewSelfSignedCertificateAuthority()
	suite.Require().NoError(err)

	cfg, err = container.PatchDocument(cfg, func(etcdCAConfig *etcd.EtcdCAConfigV1Alpha1) error {
		etcdCAConfig.CAAcceptedCAs = []string{string(extraCA.CrtPEM)}

		return nil
	})
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.EtcdRootID},
		func(res *secrets.EtcdRoot, asrt *assert.Assertions) {
			asrt.Equal(res.TypedSpec().EtcdCA, cfg.EtcdCAConfig().IssuingCA())
			asrt.Equal(
				[]*x509.PEMEncodedCertificate{
					{
						Crt: cfg.EtcdCAConfig().IssuingCA().Crt,
					},
					{
						Crt: extraCA.CrtPEM,
					},
				},
				res.TypedSpec().AcceptedCAs,
			)
		},
	)
}

func (suite *RootSuite) TestReconcileControlPlaneNoK8sEtcd() {
	cfg := suite.genConfig(
		true,
		talosconfig.TalosVersionCurrent.DisableEtcd().DisableKubernetes(),
	)

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.OSRootID},
		func(res *secrets.OSRoot, asrt *assert.Assertions) {
			asrt.Equal(res.TypedSpec().IssuingCA, cfg.Machine().Security().IssuingCA())
			asrt.Equal(
				[]*x509.PEMEncodedCertificate{
					{
						Crt: cfg.Machine().Security().IssuingCA().Crt,
					},
				},
				res.TypedSpec().AcceptedCAs,
			)
		},
	)

	rtestutils.AssertNoResource[*secrets.EtcdRoot](suite.Ctx(), suite.T(), suite.State(), secrets.EtcdRootID)
	rtestutils.AssertNoResource[*secrets.KubernetesRoot](suite.Ctx(), suite.T(), suite.State(), secrets.KubernetesRootID)
}

func (suite *RootSuite) TestReconcileWorker() {
	cfg := suite.genConfig(false, talosconfig.TalosVersionCurrent)

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.OSRootID},
		func(res *secrets.OSRoot, asrt *assert.Assertions) {
			asrt.Nil(res.TypedSpec().IssuingCA)
			asrt.True(res.TypedSpec().TrustdCheckExtKeyUsage)
			asrt.Equal(
				[]*x509.PEMEncodedCertificate{
					{
						Crt: cfg.Machine().Security().IssuingCA().Crt,
					},
				},
				res.TypedSpec().AcceptedCAs,
			)
		},
	)

	rtestutils.AssertNoResource[*secrets.EtcdRoot](suite.Ctx(), suite.T(), suite.State(), secrets.EtcdRootID)
	rtestutils.AssertNoResource[*secrets.KubernetesRoot](suite.Ctx(), suite.T(), suite.State(), secrets.KubernetesRootID)
}

func (suite *RootSuite) TestReconcileWorkerLegacyContract() {
	suite.genConfig(false, talosconfig.TalosVersion1_14)

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.OSRootID},
		func(res *secrets.OSRoot, asrt *assert.Assertions) {
			asrt.Nil(res.TypedSpec().IssuingCA)
			asrt.False(res.TypedSpec().TrustdCheckExtKeyUsage)
		},
	)
}
