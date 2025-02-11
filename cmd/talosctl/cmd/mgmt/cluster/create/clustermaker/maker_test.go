// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clustermaker //nolint:testpackage

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

// getTestOps returns the barebones options needed to initialize the clustermaker.
func getTestOps() Options {
	return Options{
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

func (p testProvisioner) GenOptions(r provision.NetworkRequest) []generate.Option {
	return []generate.Option{func(o *generate.Options) error {
		o.CNIConfig = &v1alpha1.CNIConfig{
			CNIName: "testname",
		}

		return nil
	}}
}

func (p testProvisioner) GetTalosAPIEndpoints(provision.NetworkRequest) []string {
	return []string{"talos-api-endpoint.test"}
}

func (p testProvisioner) GetInClusterKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequest, controlPlanePort int) string {
	return "https://" + nethelpers.JoinHostPort(networkReq.CIDRs[0].Addr().Next().Next().String(), controlPlanePort)
}

func (p testProvisioner) GetExternalKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequest, controlPlanePort int) string {
	return "https://" + nethelpers.JoinHostPort(networkReq.CIDRs[0].Addr().Next().Next().String(), controlPlanePort)
}

type testFields = Options

// n returns the field names of s.
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

func bundleApply(t *testing.T, opts ...bundle.Option) bundle.Options {
	options := bundle.Options{}
	for _, opt := range opts {
		if err := opt(&options); err != nil {
			t.Error("failed to apply option: ", err)
		}
	}

	return options
}

const testTalosVersion = "1.1"

func init() {
	addFieldTest("TestCriticalOptions",
		n(&tf, &tf.Workers, &tf.Controlplanes, &tf.NetworkCIDR, &tf.NetworkMTU, &tf.WorkersCpus, &tf.ControlPlaneCpus,
			&tf.ControlPlaneMemory, &tf.WorkersMemory, &tf.NetworkIPv4, &tf.RootOps, &tf.Controlplanes),
		func(t *testing.T) {
			input := getTestOps()
			cm := getFinalizedClusterMaker(t, input)
			result := cm.GetPartialClusterRequest()

			// Nodes
			workersResult := result.Nodes.WorkerNodes()
			controlsResult := result.Nodes.ControlPlaneNodes()
			assert.Equal(t, input.Controlplanes, len(controlsResult))
			assert.Equal(t, input.Workers, len(workersResult))

			for i := range input.Controlplanes {
				n := controlsResult[i]
				assert.EqualValues(t, 4000000000, n.NanoCPUs)
				assert.EqualValues(t, 4294967296, n.Memory)
				assert.Equal(t, false, n.SkipInjectingConfig)
				assert.Equal(t, machine.TypeControlPlane, n.Type)
				assert.Equal(t, "test-cluster-controlplane-"+strconv.Itoa(i+1), n.Name)
				assert.Equal(t, 1, len(n.IPs))
			}

			for i := range input.Workers {
				n := workersResult[i]
				assert.EqualValues(t, 2000000000, n.NanoCPUs)
				assert.EqualValues(t, 2147483648, n.Memory)
				assert.Equal(t, false, n.SkipInjectingConfig)
				assert.Equal(t, machine.TypeWorker, n.Type)
				assert.Equal(t, "test-cluster-worker-"+strconv.Itoa(i+1), n.Name)
				assert.Equal(t, 1, len(n.IPs))
			}

			// ClusterRequest
			assert.Equal(t, input.RootOps.ClusterName, cm.request.Name)
			assert.Equal(t, input.RootOps.StateDir, cm.request.StateDirectory)
			assert.NotZero(t, cm.request.SelfExecutable)

			// Network
			networkResult := cm.request.Network
			assert.Equal(t, 1, len(networkResult.CIDRs))
			assert.Equal(t, input.NetworkCIDR, networkResult.CIDRs[0].String())
			assert.Equal(t, 1, len(networkResult.GatewayAddrs))
			assert.Equal(t, "10.5.0.1", networkResult.GatewayAddrs[0].String())
			assert.Equal(t, input.NetworkMTU, networkResult.MTU)
			assert.Equal(t, input.RootOps.ClusterName, networkResult.Name)
			assert.Equal(t, cm.cidr4, networkResult.CIDRs[0])
			assert.Equal(t, 1, len(cm.ips))
			assert.Equal(t, 4, len(cm.ips[0]))
			assert.Equal(t, "10.5.0.2", cm.ips[0][0].String())
			assert.Equal(t, "10.5.0.5", cm.ips[0][3].String())
			assert.Equal(t, "10.5.0.2", cm.request.Nodes.ControlPlaneNodes()[0].IPs[0].String())
			assert.Equal(t, "10.5.0.5", cm.request.Nodes.WorkerNodes()[1].IPs[0].String())
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

		cm, err := newClusterMaker(Input{input, testProvisioner{}, "v0.1"})
		assert.NoError(t, err)
		err = cm.finalizeRequest()
		assert.NoError(t, err)
		result, err := generate.NewInput(input.RootOps.ClusterName, "cluster.endpoint", "k8sv1", cm.genOpts...)
		assert.NoError(t, err)

		assert.EqualValues(t, "v0.1", result.Options.VersionContract.String())
	})
	addFieldTest("TestInvalidTalosVersion", n(&tf, &tf.TalosVersion), func(t *testing.T) {
		input := getTestOps()

		_, err := New(Input{input, testProvisioner{}, "invalid"})
		assert.ErrorContains(t, err, "error parsing Talos version")
	})

	//
	// Config bundle options
	//
	addFieldTest("", n(&tf, &tf.KubernetesVersion), func(t *testing.T) {
		input := getTestOps()
		input.KubernetesVersion = "1.1.1-test"

		cm := getFinalizedClusterMaker(t, input)

		opts := bundleApply(t, cm.cfgBundleOpts...)
		assert.Equal(t, "1.1.1-test", opts.InputOptions.KubeVersion)
	})
	addFieldTest("", n(&tf, &tf.EnableKubeSpan), func(t *testing.T) {
		input := getTestOps()
		input.EnableKubeSpan = true

		cm := getFinalizedClusterMaker(t, input)

		assert.EqualValues(t, true, cm.configBundle.Init().RawV1Alpha1().MachineConfig.MachineNetwork.KubeSpan().Enabled())
	})
	addFieldTest("", n(&tf, &tf.WithJSONLogs), func(t *testing.T) {
		input := getTestOps()
		input.WithJSONLogs = true

		cm := getFinalizedClusterMaker(t, input)

		assert.EqualValues(t, "json_lines", cm.configBundle.Init().RawV1Alpha1().MachineConfig.MachineLogging.LoggingDestinations[0].LoggingFormat)
	})
	addFieldTest("", n(&tf, &tf.WireguardCIDR), func(t *testing.T) {
		input := getTestOps()
		input.WireguardCIDR = "10.1.0.0/16"

		cm := getFinalizedClusterMaker(t, input)

		assert.EqualValues(t, 1, len(cm.request.Nodes.WorkerNodes()[0].Config.RawV1Alpha1().MachineConfig.MachineNetwork.NetworkInterfaces))
		assert.EqualValues(t, 1, len(cm.request.Nodes.ControlPlaneNodes()[0].Config.RawV1Alpha1().MachineConfig.MachineNetwork.NetworkInterfaces))
		assert.EqualValues(t, 1, len(cm.request.Nodes.ControlPlaneNodes()[1].Config.RawV1Alpha1().MachineConfig.MachineNetwork.NetworkInterfaces))
		assert.EqualValues(t, "wg0", cm.request.Nodes.WorkerNodes()[0].Config.RawV1Alpha1().MachineConfig.MachineNetwork.NetworkInterfaces[0].DeviceInterface)
	})
	addFieldTest("TestConfigPatches", n(&tf, &tf.ConfigPatch, &tf.ConfigPatchControlPlane, &tf.ConfigPatchWorker), func(t *testing.T) {
		input := getTestOps()
		input.ConfigPatch = []string{`[{"op": "add", "path": "/machine/network/hostname", "value": "test-hostname"}]`}
		input.ConfigPatchControlPlane = []string{`[{"op": "add", "path": "/machine/kubelet/image", "value": "test-control"}]`}
		input.ConfigPatchWorker = []string{`[{"op": "add", "path": "/machine/kubelet/image", "value": "test-worker"}]`}

		cm := getFinalizedClusterMaker(t, input)

		assert.EqualValues(t, "test-hostname", cm.request.Nodes.WorkerNodes()[0].Config.RawV1Alpha1().MachineConfig.Network().Hostname())
		assert.EqualValues(t, "test-control", cm.request.Nodes.ControlPlaneNodes()[0].Config.RawV1Alpha1().MachineConfig.Kubelet().Image())
		assert.EqualValues(t, "test-worker", cm.request.Nodes.WorkerNodes()[0].Config.RawV1Alpha1().MachineConfig.Kubelet().Image())
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

		getFinalizedClusterMaker(t, input)
	})
	addFieldTest("TestPostCreateSkipFields", n(&tf, &tf.SkipInjectingConfig, &tf.SkipK8sNodeReadinessCheck, &tf.SkipKubeconfig), func(t *testing.T) {
		input := getTestOps()
		input.SkipK8sNodeReadinessCheck = true
		input.SkipKubeconfig = true

		getFinalizedClusterMaker(t, input)
	})

	addFieldTest("", n(&tf, &tf.NetworkIPv6), func(t *testing.T) {
		input := getTestOps()
		input.NetworkIPv6 = true

		cm := getFinalizedClusterMaker(t, input)

		nodes := slices.Concat(cm.request.Nodes.ControlPlaneNodes(), cm.request.Nodes.WorkerNodes())

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
		cm := getFinalizedClusterMaker(t, input)

		err := cm.configBundle.Write(dir, encoder.CommentsDisabled, machine.TypeControlPlane, machine.TypeWorker)
		assert.NoError(t, err)

		input = getTestOps()
		input.InputDir = dir
		cm = getFinalizedClusterMaker(t, input)

		assert.EqualValues(t, "https://test.mirror", cm.request.Nodes.WorkerNodes()[0].Config.RawV1Alpha1().MachineConfig.Registries().Mirrors()["test.test"].Endpoints()[0])
		assert.EqualValues(t, "https://test.mirror", cm.request.Nodes.ControlPlaneNodes()[0].Config.RawV1Alpha1().MachineConfig.Registries().Mirrors()["test.test"].Endpoints()[0])
	})
}

func getFinalizedClusterMaker(t *testing.T, input Options) clusterMaker {
	cm, err := newClusterMaker(Input{input, testProvisioner{}, testTalosVersion})
	assert.NoError(t, err)
	err = cm.finalizeRequest()
	assert.NoError(t, err)

	return cm
}

func getGenOpts(t *testing.T, input Options) generate.Options {
	cm, err := newClusterMaker(Input{input, testProvisioner{}, testTalosVersion})
	assert.NoError(t, err)
	err = cm.finalizeRequest()
	assert.NoError(t, err)
	result, err := generate.NewInput(input.RootOps.ClusterName, "cluster.endpoint", "k8sv1", cm.genOpts...)
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

func TestAllOptionFields(t *testing.T) {
	for _, fieldTest := range fieldTests {
		t.Run(fieldTest.name, fieldTest.test)
	}

	type testField = struct {
		name   string
		tested bool
	}

	typeof := reflect.TypeOf(Options{})
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
	assert.Equal(t, 0, len(untested), "all fields of Options struct need to be tested. Untested fields: ", untestedNames)
}

func TestProvisionerGenOptions(t *testing.T) {
	input := getTestOps()

	options := getGenOpts(t, input)

	assert.EqualValues(t, "testname", options.CNIConfig.CNIName)
}
