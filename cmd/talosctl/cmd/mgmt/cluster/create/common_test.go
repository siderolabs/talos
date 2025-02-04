// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create //nolint:testpackage

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/provision"
)

// getTestOps returns the barebones options needed to run getBase().
func getTestOps() CommonOps {
	return CommonOps{
		RootOps: &cluster.CmdOps{
			ProvisionerName: "test-provisioner",
			StateDir:        "/state-dir",
			ClusterName:     "test-cluster",
		},
		Workers:            2,
		Controlplanes:      2,
		NetworkCIDR:        "10.5.0.0/24",
		NetworkMTU:         1500,
		WorkersCpus:        "2.0",
		ControlPlaneCpus:   "4",
		ControlPlaneMemory: 4096,
		WorkersMemory:      2048,
		NetworkIPv4:        true,
	}
}

type testProvisioner struct {
	provision.Provisioner
}

func (p testProvisioner) GenOptions(r provision.NetworkRequestBase) []generate.Option {
	return []generate.Option{func(o *generate.Options) error {
		o.CNIConfig = &v1alpha1.CNIConfig{
			CNIName: "testname",
		}

		return nil
	}}
}

func (p testProvisioner) GetTalosAPIEndpoints(provision.NetworkRequestBase) []string {
	return []string{"talos-api-endpoint.test"}
}

func (p testProvisioner) GetInClusterKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequestBase, controlPlanePort int) string {
	return "https://" + nethelpers.JoinHostPort(networkReq.CIDRs[0].Addr().Next().Next().String(), controlPlanePort)
}

func (p testProvisioner) GetExternalKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequestBase, controlPlanePort int) string {
	return "https://" + nethelpers.JoinHostPort(networkReq.CIDRs[0].Addr().Next().Next().String(), controlPlanePort)
}

func getTalosTestVersion() string {
	return "1.1"
}

type testFields = CommonOps

// n returns the field names.
func n(s any, fields ...any) []string {
	var names []string

	for _, f := range fields {
		rv := reflect.ValueOf(s).Elem()
		for i := range rv.NumField() {
			fv := rv.Field(i)
			fp := fv.Addr().Interface()

			if f == fp {
				names = append(names, rv.Type().Field(i).Name)
			}
		}
	}

	return names
}

var tf = testFields{}

func returnNoAdditionalOpts(cOps CommonOps, base clusterCreateBase) (additional additionalOptions, err error) {
	return
}

func bundleApply(t *testing.T, opts ...bundle.Option) bundle.Options {
	options := bundle.Options{}
	for _, opt := range opts {
		if err := opt(&options); err != nil {
			t.Error("failed to apply option: ", err)
		}
	}

	return options
}

func init() {
	addFieldTest("TestCriticalOptions",
		n(&tf, &tf.Workers, &tf.Controlplanes, &tf.NetworkCIDR, &tf.NetworkMTU, &tf.WorkersCpus, &tf.ControlPlaneCpus,
			&tf.ControlPlaneMemory, &tf.WorkersMemory, &tf.NetworkIPv4, &tf.RootOps, &tf.Controlplanes),
		func(t *testing.T) {
			input := getTestOps()
			result, err := getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
			assert.NoError(t, err)

			// Nodes
			workersResult := result.clusterRequest.Workers
			controlsResult := result.clusterRequest.Controlplanes
			assert.Equal(t, input.Controlplanes, len(controlsResult))
			assert.Equal(t, input.Workers, len(workersResult))

			for i := range input.Controlplanes {
				n := controlsResult[i]
				assert.Equal(t, i, n.Index)
				assert.EqualValues(t, 4000000000, n.NanoCPUs)
				assert.EqualValues(t, 4294967296, n.Memory)
				assert.Equal(t, false, n.SkipInjectingConfig)
				assert.Equal(t, machine.TypeControlPlane, n.Type)
				assert.Equal(t, "test-cluster-controlplane-"+strconv.Itoa(i+1), n.Name)
				assert.Equal(t, 1, len(n.IPs))
			}

			for i := range input.Workers {
				n := workersResult[i]
				assert.Equal(t, i+input.Controlplanes, n.Index)
				assert.EqualValues(t, 2000000000, n.NanoCPUs)
				assert.EqualValues(t, 2147483648, n.Memory)
				assert.Equal(t, false, n.SkipInjectingConfig)
				assert.Equal(t, machine.TypeWorker, n.Type)
				assert.Equal(t, "test-cluster-worker-"+strconv.Itoa(i+1), n.Name)
				assert.Equal(t, 1, len(n.IPs))
			}

			// ClusterRequest
			clusterReqResult := result.clusterRequest
			assert.Equal(t, input.RootOps.ClusterName, clusterReqResult.Name)
			assert.Equal(t, input.RootOps.StateDir, clusterReqResult.StateDirectory)
			assert.NotZero(t, clusterReqResult.SelfExecutable)

			// Network
			networkResult := clusterReqResult.Network
			assert.Equal(t, 1, len(networkResult.CIDRs))
			assert.Equal(t, input.NetworkCIDR, networkResult.CIDRs[0].String())
			assert.Equal(t, 1, len(networkResult.GatewayAddrs))
			assert.Equal(t, "10.5.0.1", networkResult.GatewayAddrs[0].String())
			assert.Equal(t, input.NetworkMTU, networkResult.MTU)
			assert.Equal(t, input.RootOps.ClusterName, networkResult.Name)
			assert.Equal(t, result.cidr4, networkResult.CIDRs[0])
			assert.Equal(t, 1, len(result.ips))
			assert.Equal(t, 4, len(result.ips[0]))
			assert.Equal(t, "10.5.0.2", result.ips[0][0].String())
			assert.Equal(t, "10.5.0.5", result.ips[0][3].String())
			assert.Equal(t, "10.5.0.2", result.clusterRequest.Controlplanes[0].IPs[0].String())
			assert.Equal(t, "10.5.0.5", result.clusterRequest.Workers[1].IPs[0].String())
		})

	//
	// Generate Options
	//
	addFieldTest("TestRegistryMirrors", n(&tf, &tf.RegistryMirrors, &tf.RegistryInsecure), func(t *testing.T) {
		input := getTestOps()
		input.RegistryMirrors = []string{"test.test=https://test.mirror", "insecure.test=https://insecure.mirror"}
		input.RegistryInsecure = []string{"insecure.test"}

		options := getGenOpts(t, input)

		assert.Equal(t, 2, len(options.RegistryMirrors))
		assert.Equal(t, "https://test.mirror", options.RegistryMirrors["test.test"].MirrorEndpoints[0])
		assert.Equal(t, "https://insecure.mirror", options.RegistryMirrors["insecure.test"].MirrorEndpoints[0])
		assert.Equal(t, true, options.RegistryConfig["insecure.test"].RegistryTLS.InsecureSkipVerify())
	})
	addFieldTest("", n(&tf, &tf.ConfigDebug), func(t *testing.T) {
		input := getTestOps()
		input.ConfigDebug = true

		options := getGenOpts(t, input)

		assert.Equal(t, true, options.Debug)
	})
	addFieldTest("", n(&tf, &tf.DNSDomain), func(t *testing.T) {
		input := getTestOps()
		input.DNSDomain = "test.dns"

		options := getGenOpts(t, input)

		assert.Equal(t, "test.dns", options.DNSDomain)
	})
	addFieldTest("", n(&tf, &tf.EnableClusterDiscovery), func(t *testing.T) {
		input := getTestOps()
		input.EnableClusterDiscovery = true

		options := getGenOpts(t, input)

		assert.Equal(t, true, *options.DiscoveryEnabled)
	})
	addFieldTest("", n(&tf, &tf.CustomCNIUrl), func(t *testing.T) {
		input := getTestOps()
		input.CustomCNIUrl = "test.url"

		options := getGenOpts(t, input)

		assert.EqualValues(t, []string{"test.url"}, options.CNIConfig.CNIUrls)
	})
	addFieldTest("", n(&tf, &tf.ForceInitNodeAsEndpoint), func(t *testing.T) {
		input := getTestOps()
		input.ForceInitNodeAsEndpoint = true

		options := getGenOpts(t, input)

		assert.EqualValues(t, 1, len(options.EndpointList))
		assert.EqualValues(t, "10.5.0.2", options.EndpointList[0])
	})
	addFieldTest("", n(&tf, &tf.ControlPlanePort), func(t *testing.T) {
		input := getTestOps()
		input.ControlPlanePort = 1111

		options := getGenOpts(t, input)

		assert.EqualValues(t, 1111, options.LocalAPIServerPort)
	})
	addFieldTest("", n(&tf, &tf.KubePrismPort), func(t *testing.T) {
		input := getTestOps()
		input.KubePrismPort = 2222

		options := getGenOpts(t, input)

		assert.EqualValues(t, 2222, *options.KubePrismPort.Ptr())
	})
	addFieldTest("", n(&tf, &tf.ForceEndpoint), func(t *testing.T) {
		input := getTestOps()
		input.ForceEndpoint = "test"

		options := getGenOpts(t, input)

		assert.EqualValues(t, 1, len(options.EndpointList))
		assert.EqualValues(t, "test", options.EndpointList[0])
		assert.EqualValues(t, 1, len(options.AdditionalSubjectAltNames))
		assert.EqualValues(t, "test", options.AdditionalSubjectAltNames[0])
	})
	addFieldTest("", n(&tf, &tf.TalosVersion), func(t *testing.T) {
		input := getTestOps()
		getTalosTestVersion := func() string {
			return "v0.1"
		}
		getAdditionalOpts := func(cOps CommonOps, base clusterCreateBase) (additional additionalOptions, err error) {
			assert.Equal(t, "v0.1", cOps.TalosVersion, "should pass correct talos version in get additional options callback")

			return
		}

		baseResult, err := _getBase(input, testProvisioner{}, getTalosTestVersion, getAdditionalOpts)
		assert.NoError(t, err)
		result, err := generate.NewInput(input.RootOps.ClusterName, "cluster.endpoint", "k8sv1", baseResult.genOptions...)
		assert.NoError(t, err)

		assert.EqualValues(t, "v0.1", result.Options.VersionContract.String())

		getTalosTestVersion = func() string {
			return "invalid"
		}
		_, err = _getBase(input, testProvisioner{}, getTalosTestVersion, getAdditionalOpts)
		assert.ErrorContains(t, err, "error parsing Talos version")
	})

	//
	// Config bundle options
	//
	addFieldTest("", n(&tf, &tf.KubernetesVersion), func(t *testing.T) {
		input := getTestOps()
		input.KubernetesVersion = "1.1.1-test"

		result, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)
		opts := bundleApply(t, result.configBundleOpts...)
		assert.Equal(t, "1.1.1-test", opts.InputOptions.KubeVersion)
	})
	addFieldTest("", n(&tf, &tf.EnableKubeSpan), func(t *testing.T) {
		input := getTestOps()
		input.EnableKubeSpan = true

		result, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)

		assert.EqualValues(t, true, result.configBundle.Init().RawV1Alpha1().MachineConfig.MachineNetwork.KubeSpan().Enabled())
	})
	addFieldTest("", n(&tf, &tf.WithJSONLogs), func(t *testing.T) {
		input := getTestOps()
		input.WithJSONLogs = true

		result, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)

		assert.EqualValues(t, "json_lines", result.configBundle.Init().RawV1Alpha1().MachineConfig.MachineLogging.LoggingDestinations[0].LoggingFormat)
	})
	addFieldTest("", n(&tf, &tf.WireguardCIDR), func(t *testing.T) {
		input := getTestOps()
		input.WireguardCIDR = "10.1.0.0/16"

		result, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)

		assert.EqualValues(t, 1, len(result.base.clusterRequest.Workers[0].Config.RawV1Alpha1().MachineConfig.MachineNetwork.NetworkInterfaces))
		assert.EqualValues(t, 1, len(result.base.clusterRequest.Controlplanes[0].Config.RawV1Alpha1().MachineConfig.MachineNetwork.NetworkInterfaces))
		assert.EqualValues(t, 1, len(result.base.clusterRequest.Controlplanes[1].Config.RawV1Alpha1().MachineConfig.MachineNetwork.NetworkInterfaces))
		assert.EqualValues(t, "wg0", result.base.clusterRequest.Workers[0].Config.RawV1Alpha1().MachineConfig.MachineNetwork.NetworkInterfaces[0].DeviceInterface)
	})
	addFieldTest("TestConfigPatches", n(&tf, &tf.ConfigPatch, &tf.ConfigPatchControlPlane, &tf.ConfigPatchWorker), func(t *testing.T) {
		input := getTestOps()
		input.ConfigPatch = []string{`[{"op": "add", "path": "/machine/network/hostname", "value": "test-hostname"}]`}
		input.ConfigPatchControlPlane = []string{`[{"op": "add", "path": "/machine/kubelet/image", "value": "test-control"}]`}
		input.ConfigPatchWorker = []string{`[{"op": "add", "path": "/machine/kubelet/image", "value": "test-worker"}]`}

		result, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)

		assert.EqualValues(t, "test-hostname", result.base.clusterRequest.Workers[0].Config.RawV1Alpha1().MachineConfig.Network().Hostname())
		assert.EqualValues(t, "test-control", result.base.clusterRequest.Controlplanes[0].Config.RawV1Alpha1().MachineConfig.Kubelet().Image())
		assert.EqualValues(t, "test-worker", result.base.clusterRequest.Workers[0].Config.RawV1Alpha1().MachineConfig.Kubelet().Image())
	})

	// Most of the logic is in the post create part so these two are just smoke tests
	addFieldTest("TestPostCreateFields", n(
		&tf, &tf.ApplyConfigEnabled, &tf.ClusterWait, &tf.ClusterWaitTimeout, &tf.WithInitNode, &tf.Talosconfig,
	), func(t *testing.T) {
		input := getTestOps()
		input.ApplyConfigEnabled = true
		input.ClusterWait = true
		input.ClusterWaitTimeout = 1000
		input.WithInitNode = true
		input.Talosconfig = "test-conf"

		_, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)
	})
	addFieldTest("TestPostCreateSkipFields", n(&tf, &tf.SkipInjectingConfig, &tf.SkipK8sNodeReadinessCheck, &tf.SkipKubeconfig), func(t *testing.T) {
		input := getTestOps()
		input.SkipK8sNodeReadinessCheck = true
		input.SkipKubeconfig = true

		_, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)
	})

	addFieldTest("", n(&tf, &tf.NetworkIPv6), func(t *testing.T) {
		input := getTestOps()
		input.NetworkIPv6 = true

		result, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)

		nodes := slices.Concat(result.base.clusterRequest.Controlplanes, result.base.clusterRequest.Workers)

		for _, n := range nodes {
			assert.Equal(t, 2, len(n.IPs))
			assert.Equal(t, true, n.IPs[1].Is6())
		}

		assert.Equal(t, "10.5.0.2", nodes[0].IPs[0].String())
		assert.Equal(t, "fd74:616c:a05::2", nodes[0].IPs[1].String())
		assert.Equal(t, "10.5.0.5", nodes[3].IPs[0].String())
		assert.Equal(t, "fd74:616c:a05::5", nodes[3].IPs[1].String())
	})
	addFieldTest("", n(&tf, &tf.InputDir), func(t *testing.T) {
		dir := t.TempDir()
		input := getTestOps()
		input.RootOps.StateDir = dir
		input.RegistryMirrors = []string{"test.test=https://test.mirror"}
		result, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)
		err = result.configBundle.Write(dir, encoder.CommentsDisabled, machine.TypeControlPlane, machine.TypeWorker)
		assert.NoError(t, err)

		input = getTestOps()
		input.InputDir = dir
		result, err = _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
		assert.NoError(t, err)
		assert.EqualValues(t, "https://test.mirror", result.base.clusterRequest.Workers[0].Config.RawV1Alpha1().MachineConfig.Registries().Mirrors()["test.test"].Endpoints()[0])
		assert.EqualValues(t, "https://test.mirror", result.base.clusterRequest.Controlplanes[0].Config.RawV1Alpha1().MachineConfig.Registries().Mirrors()["test.test"].Endpoints()[0])
	})
}

func getGenOpts(t *testing.T, input CommonOps) generate.Options {
	baseResult, err := _getBase(input, testProvisioner{}, getTalosTestVersion, returnNoAdditionalOpts)
	assert.NoError(t, err)
	result, err := generate.NewInput(input.RootOps.ClusterName, "cluster.endpoint", "k8sv1", baseResult.genOptions...)
	assert.NoError(t, err)

	return result.Options
}

func addFieldTest(testName string, fieldNames []string, test func(t *testing.T)) {
	testedFields = append(testedFields, fieldNames...)
	name := fmt.Sprintf("Test%sField", fieldNames[0])

	if testName != "" {
		name = testName
	}

	if len(fieldNames) > 1 && testName == "" {
		panic("no test name specified with multiple fields tested")
	}

	fieldTests = append(fieldTests, fieldTest{fieldNames, test, name})
}

var fieldTests []fieldTest

type fieldTest = struct {
	fieldNames []string
	test       func(t *testing.T)
	name       string
}

var testedFields = []string{}

func TestAllCommonOptionFields(t *testing.T) {
	for _, fieldTest := range fieldTests {
		t.Run(fieldTest.name, fieldTest.test)
	}

	type testField = struct {
		name   string
		tested bool
	}

	typeof := reflect.TypeOf(CommonOps{})
	allFields := make([]testField, typeof.NumField())

	for i := range typeof.NumField() {
		allFields[i].name = typeof.Field(i).Name
	}

	for _, testedField := range testedFields {
		allFields = xslices.Map(allFields, func(tf testField) testField {
			if tf.name == testedField {
				tf.tested = true
			}

			return tf
		})
	}

	untested := xslices.Filter(allFields, func(tf testField) bool { return !tf.tested })
	untestedNames := xslices.Map(untested, func(tf testField) string { return tf.name })
	assert.Equal(t, 0, len(untested), "all fields of CommonOptions need to be tested. Untested fields: ", untestedNames)
}

func TestProvisionerGenOptions(t *testing.T) {
	input := getTestOps()

	options := getGenOpts(t, input)

	assert.EqualValues(t, "testname", options.CNIConfig.CNIName)
}
