// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package base

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/cluster/check"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/access"
)

// APISuite is a base suite for API tests.
type APISuite struct {
	suite.Suite
	TalosSuite

	Client      *client.Client
	Talosconfig *clientconfig.Config
}

// SetupSuite initializes Talos API client.
func (apiSuite *APISuite) SetupSuite() {
	var err error

	apiSuite.Talosconfig, err = clientconfig.Open(apiSuite.TalosConfig)
	apiSuite.Require().NoError(err)

	opts := []client.OptionFunc{
		client.WithConfig(apiSuite.Talosconfig),
	}

	if apiSuite.Endpoint != "" {
		opts = append(opts, client.WithEndpoints(apiSuite.Endpoint))
	}

	apiSuite.Client, err = client.New(context.TODO(), opts...)
	apiSuite.Require().NoError(err)

	// clear any connection refused errors left after the previous tests
	nodes := apiSuite.DiscoverNodeInternalIPs(context.TODO())

	if len(nodes) > 0 {
		// grpc might trigger backoff on reconnect attempts, so make sure we clear them
		apiSuite.ClearConnectionRefused(context.Background(), nodes...)
	}
}

// DiscoverNodes provides list of Talos nodes in the cluster.
//
// As there's no way to provide this functionality via Talos API, it works the following way:
// 1. If there's a provided cluster info, it's used.
// 2. If integration test was compiled with k8s support, k8s is used.
//
// The passed ctx is additionally limited to one minute.
func (apiSuite *APISuite) DiscoverNodes(ctx context.Context) cluster.Info {
	discoveredNodes := apiSuite.TalosSuite.DiscoverNodes(ctx)
	if discoveredNodes != nil {
		return discoveredNodes
	}

	var err error

	var ctxCancel context.CancelFunc
	ctx, ctxCancel = context.WithTimeout(ctx, time.Minute)

	defer ctxCancel()

	apiSuite.discoveredNodes, err = discoverNodesK8s(ctx, apiSuite.Client, &apiSuite.TalosSuite)
	apiSuite.Require().NoError(err, "k8s discovery failed")

	if apiSuite.discoveredNodes == nil {
		// still no nodes, skip the test
		apiSuite.T().Skip("no nodes were discovered")
	}

	return apiSuite.discoveredNodes
}

// DiscoverNodeInternalIPs provides list of Talos node internal IPs in the cluster.
func (apiSuite *APISuite) DiscoverNodeInternalIPs(ctx context.Context) []string {
	nodes := apiSuite.DiscoverNodes(ctx).Nodes()

	return mapNodeInfosToInternalIPs(nodes)
}

// DiscoverNodeInternalIPsByType provides list of Talos node internal IPs in the cluster for given machine type.
func (apiSuite *APISuite) DiscoverNodeInternalIPsByType(ctx context.Context, machineType machine.Type) []string {
	nodesByType := apiSuite.DiscoverNodes(ctx).NodesByType(machineType)

	return mapNodeInfosToInternalIPs(nodesByType)
}

// RandomDiscoveredNodeInternalIP returns the internal IP a random node of the specified type (or any type if no types are specified).
func (apiSuite *APISuite) RandomDiscoveredNodeInternalIP(types ...machine.Type) string {
	nodeInfo := apiSuite.DiscoverNodes(context.TODO())

	var nodes []cluster.NodeInfo

	if len(types) == 0 {
		nodeInfos := nodeInfo.Nodes()

		nodes = nodeInfos
	} else {
		for _, t := range types {
			nodeInfosByType := nodeInfo.NodesByType(t)

			nodes = append(nodes, nodeInfosByType...)
		}
	}

	apiSuite.Require().NotEmpty(nodes)

	return nodes[rand.Intn(len(nodes))].InternalIP.String()
}

// Capabilities describes current cluster allowed actions.
type Capabilities struct {
	RunsTalosKernel bool
	SupportsReboot  bool
	SupportsRecover bool
	SecureBooted    bool
}

// Capabilities returns a set of capabilities to skip tests for different environments.
func (apiSuite *APISuite) Capabilities() Capabilities {
	v, err := apiSuite.Client.Version(context.Background())
	apiSuite.Require().NoError(err)

	caps := Capabilities{}

	if v.Messages[0].Platform != nil {
		switch v.Messages[0].Platform.Mode {
		case runtime.ModeContainer.String():
		default:
			caps.RunsTalosKernel = true
			caps.SupportsReboot = true
			caps.SupportsRecover = true
		}
	}

	ctx := context.Background()
	ctx, ctxCancel := context.WithTimeout(ctx, 2*time.Minute)

	defer ctxCancel()

	securityResource, err := safe.StateWatchFor[*runtimeres.SecurityState](
		ctx,
		apiSuite.Client.COSI,
		runtimeres.NewSecurityStateSpec(runtimeres.NamespaceName).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
	)
	apiSuite.Require().NoError(err)

	caps.SecureBooted = securityResource.TypedSpec().SecureBoot

	return caps
}

// AssertClusterHealthy verifies that cluster is healthy using provisioning checks.
func (apiSuite *APISuite) AssertClusterHealthy(ctx context.Context) {
	if apiSuite.Cluster == nil {
		// can't assert if cluster state was provided
		apiSuite.T().Skip("cluster health can't be verified when cluster state is not provided")
	}

	clusterAccess := access.NewAdapter(apiSuite.Cluster, provision.WithTalosClient(apiSuite.Client))
	defer clusterAccess.Close() //nolint:errcheck

	apiSuite.Require().NoError(check.Wait(ctx, clusterAccess, append(check.DefaultClusterChecks(), check.ExtraClusterChecks()...), check.StderrReporter()))
}

// ReadBootID reads node boot_id.
//
// Context provided might have specific node attached for API call.
func (apiSuite *APISuite) ReadBootID(ctx context.Context) (string, error) {
	// set up a short timeout around boot_id read calls to work around
	// cases when rebooted node doesn't answer for a long time on requests
	reqCtx, reqCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	defer reqCtxCancel()

	reader, err := apiSuite.Client.Read(reqCtx, "/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}

	defer reader.Close() //nolint:errcheck

	body, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	bootID := strings.TrimSpace(string(body))

	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return "", err
	}

	return bootID, reader.Close()
}

// ReadBootIDWithRetry reads node boot_id.
//
// Context provided might have specific node attached for API call.
func (apiSuite *APISuite) ReadBootIDWithRetry(ctx context.Context, timeout time.Duration) string {
	var bootID string

	apiSuite.Require().NoError(retry.Constant(timeout, retry.WithUnits(time.Millisecond*1000)).Retry(
		func() error {
			var err error

			bootID, err = apiSuite.ReadBootID(ctx)
			if err != nil {
				return retry.ExpectedError(err)
			}

			if bootID == "" {
				return retry.ExpectedErrorf("boot id is empty")
			}

			return nil
		},
	))

	return bootID
}

// AssertRebooted verifies that node got rebooted as result of running some API call.
//
// Verification happens via reading boot_id of the node.
func (apiSuite *APISuite) AssertRebooted(ctx context.Context, node string, rebootFunc func(nodeCtx context.Context) error, timeout time.Duration) {
	apiSuite.AssertRebootedNoChecks(ctx, node, rebootFunc, timeout)

	apiSuite.WaitForBootDone(ctx)

	if apiSuite.Cluster != nil {
		// without cluster state we can't do deep checks, but basic reboot test still works
		// NB: using `ctx` here to have client talking to init node by default
		apiSuite.AssertClusterHealthy(ctx)
	}
}

// AssertRebootedNoChecks waits for node to be rebooted without waiting for cluster to become healthy afterwards.
func (apiSuite *APISuite) AssertRebootedNoChecks(ctx context.Context, node string, rebootFunc func(nodeCtx context.Context) error, timeout time.Duration) {
	// timeout for single node Reset
	ctx, ctxCancel := context.WithTimeout(ctx, timeout)
	defer ctxCancel()

	nodeCtx := client.WithNodes(ctx, node)

	var (
		bootIDBefore string
		err          error
	)

	err = retry.Constant(time.Minute * 5).Retry(func() error {
		// read boot_id before reboot
		bootIDBefore, err = apiSuite.ReadBootID(nodeCtx)

		if err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	})

	apiSuite.Require().NoError(err)

	apiSuite.Assert().NoError(rebootFunc(nodeCtx))

	apiSuite.AssertBootIDChanged(nodeCtx, bootIDBefore, node, timeout)
}

// AssertBootIDChanged waits until node boot id changes.
func (apiSuite *APISuite) AssertBootIDChanged(nodeCtx context.Context, bootIDBefore, node string, timeout time.Duration) {
	apiSuite.Assert().NotEmpty(bootIDBefore)

	apiSuite.Require().NoError(retry.Constant(timeout).Retry(func() error {
		requestCtx, requestCtxCancel := context.WithTimeout(nodeCtx, time.Second)
		defer requestCtxCancel()

		bootIDAfter, err := apiSuite.ReadBootID(requestCtx)
		if err != nil {
			// API might be unresponsive during reboot
			return retry.ExpectedError(err)
		}

		if bootIDAfter == bootIDBefore {
			// bootID should be different after reboot
			return retry.ExpectedError(fmt.Errorf("bootID didn't change for node %q: before %s, after %s", node, bootIDBefore, bootIDAfter))
		}

		return nil
	}))
}

// WaitForBootDone waits for boot phase done event.
func (apiSuite *APISuite) WaitForBootDone(ctx context.Context) {
	apiSuite.WaitForSequenceDone(
		ctx,
		runtime.SequenceBoot,
		apiSuite.DiscoverNodeInternalIPs(ctx)...,
	)
}

// WaitForSequenceDone waits for sequence done event.
func (apiSuite *APISuite) WaitForSequenceDone(ctx context.Context, sequence runtime.Sequence, nodes ...string) {
	nodesNotDone := make(map[string]struct{})

	for _, node := range nodes {
		nodesNotDone[node] = struct{}{}
	}

	apiSuite.Require().NoError(retry.Constant(5*time.Minute, retry.WithUnits(time.Second*10)).Retry(func() error {
		eventsCtx, cancel := context.WithTimeout(client.WithNodes(ctx, nodes...), 5*time.Second)
		defer cancel()

		err := apiSuite.Client.EventsWatch(eventsCtx, func(ch <-chan client.Event) {
			defer cancel()

			for event := range ch {
				if msg, ok := event.Payload.(*machineapi.SequenceEvent); ok {
					if msg.GetAction() == machineapi.SequenceEvent_STOP && msg.GetSequence() == sequence.String() {
						delete(nodesNotDone, event.Node)

						if len(nodesNotDone) == 0 {
							return
						}
					}
				}
			}
		}, client.WithTailEvents(-1))
		if err != nil {
			return retry.ExpectedError(err)
		}

		if len(nodesNotDone) > 0 {
			return retry.ExpectedErrorf("nodes %#v sequence %s is not completed", nodesNotDone, sequence.String())
		}

		return nil
	}))
}

// ClearConnectionRefused clears cached connection refused errors which might be left after node reboot.
func (apiSuite *APISuite) ClearConnectionRefused(ctx context.Context, nodes ...string) {
	ctx, cancel := context.WithTimeout(ctx, backoff.DefaultConfig.MaxDelay)
	defer cancel()

	controlPlaneNodes := apiSuite.DiscoverNodes(ctx).NodesByType(machine.TypeControlPlane)
	initNodes := apiSuite.DiscoverNodes(ctx).NodesByType(machine.TypeInit)

	numMasterNodes := len(controlPlaneNodes) + len(initNodes)
	if numMasterNodes == 0 {
		numMasterNodes = 3
	}

	apiSuite.Require().NoError(retry.Constant(backoff.DefaultConfig.MaxDelay, retry.WithUnits(time.Second)).Retry(func() error {
		for i := 0; i < numMasterNodes; i++ {
			_, err := apiSuite.Client.Version(client.WithNodes(ctx, nodes...))
			if err == nil {
				continue
			}

			if client.StatusCode(err) == codes.Unavailable {
				return retry.ExpectedError(err)
			}

			if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "connection reset by peer") {
				return retry.ExpectedError(err)
			}

			return err
		}

		return nil
	}))
}

// HashKubeletCert returns hash of the kubelet certificate file.
//
// This function can be used to verify that the node ephemeral partition got wiped.
func (apiSuite *APISuite) HashKubeletCert(ctx context.Context, node string) (string, error) {
	reqCtx, reqCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	defer reqCtxCancel()

	reqCtx = client.WithNodes(reqCtx, node)

	reader, err := apiSuite.Client.Read(reqCtx, "/var/lib/kubelet/pki/kubelet-client-current.pem")
	if err != nil {
		return "", err
	}

	defer reader.Close() //nolint:errcheck

	hash := sha256.New()

	_, err = io.Copy(hash, reader)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), reader.Close()
}

// ReadConfigFromNode reads machine configuration from the node.
func (apiSuite *APISuite) ReadConfigFromNode(nodeCtx context.Context) (config.Provider, error) {
	// Load the current node machine config
	cfgData := new(bytes.Buffer)

	reader, err := apiSuite.Client.Read(nodeCtx, constants.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error creating reader: %w", err)
	}
	defer reader.Close() //nolint:errcheck

	if _, err = io.Copy(cfgData, reader); err != nil {
		return nil, fmt.Errorf("error reading: %w", err)
	}

	provider, err := configloader.NewFromBytes(cfgData.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return provider, nil
}

// UserDisks returns list of user disks on with size greater than sizeGreaterThanGB and not having any partitions present.
//
//nolint:gocyclo
func (apiSuite *APISuite) UserDisks(ctx context.Context, node string, sizeGreaterThanGB int) ([]string, error) {
	nodeCtx := client.WithNodes(ctx, node)

	resp, err := apiSuite.Client.Disks(nodeCtx)
	if err != nil {
		return nil, err
	}

	var disks []string

	blockDeviceInUse := func(deviceName string) (bool, error) {
		devicePart := strings.Split(deviceName, "/dev/")[1]

		// https://unix.stackexchange.com/questions/111779/how-to-find-out-easily-whether-a-block-device-or-a-part-of-it-is-mounted-someh
		// this was the only easy way I could find to check if the block device is already in use by something like raid
		stream, err := apiSuite.Client.LS(nodeCtx, &machineapi.ListRequest{
			Root: fmt.Sprintf("/sys/block/%s/holders", devicePart),
		})
		if err != nil {
			return false, err
		}

		counter := 0

		if err = helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
			counter++

			return nil
		}); err != nil {
			return false, err
		}

		if counter > 1 {
			return true, nil
		}

		return false, nil
	}

	for _, msg := range resp.Messages {
		for _, disk := range msg.Disks {
			if disk.SystemDisk {
				continue
			}

			blockDeviceUsed, err := blockDeviceInUse(disk.DeviceName)
			if err != nil {
				return nil, err
			}

			if disk.Size > uint64(sizeGreaterThanGB)*1024*1024*1024 && !blockDeviceUsed {
				disks = append(disks, disk.DeviceName)
			}
		}
	}

	return disks, nil
}

// AssertServicesRunning verifies that services are running on the node.
func (apiSuite *APISuite) AssertServicesRunning(ctx context.Context, node string, serviceStatus map[string]string) {
	nodeCtx := client.WithNode(ctx, node)

	for svc, state := range serviceStatus {
		resp, err := apiSuite.Client.ServiceInfo(nodeCtx, svc)
		apiSuite.Require().NoError(err)
		apiSuite.Require().NotNil(resp, "expected service %s to be registered", svc)

		for _, svcInfo := range resp {
			apiSuite.Require().Equal(state, svcInfo.Service.State, "expected service %s to have state %s", svc, state)
		}
	}
}

// AssertExpectedModules verifies that expected kernel modules are loaded on the node.
func (apiSuite *APISuite) AssertExpectedModules(ctx context.Context, node string, expectedModules map[string]string) {
	nodeCtx := client.WithNode(ctx, node)

	fileReader, err := apiSuite.Client.Read(nodeCtx, "/proc/modules")
	apiSuite.Require().NoError(err)

	defer func() {
		apiSuite.Require().NoError(fileReader.Close())
	}()

	scanner := bufio.NewScanner(fileReader)

	var loadedModules []string

	for scanner.Scan() {
		loadedModules = append(loadedModules, strings.Split(scanner.Text(), " ")[0])
	}
	apiSuite.Require().NoError(scanner.Err())

	fileReader, err = apiSuite.Client.Read(nodeCtx, fmt.Sprintf("/lib/modules/%s/modules.dep", constants.DefaultKernelVersion))
	apiSuite.Require().NoError(err)

	defer func() {
		apiSuite.Require().NoError(fileReader.Close())
	}()

	scanner = bufio.NewScanner(fileReader)

	var modulesDep []string

	for scanner.Scan() {
		modulesDep = append(modulesDep, filepath.Base(strings.Split(scanner.Text(), ":")[0]))
	}
	apiSuite.Require().NoError(scanner.Err())

	for module, moduleDep := range expectedModules {
		apiSuite.Require().Contains(loadedModules, module, "expected %s to be loaded", module)
		apiSuite.Require().Contains(modulesDep, moduleDep, "expected %s to be in modules.dep", moduleDep)
	}
}

// PatchV1Alpha1Config patches v1alpha1 config in the config provider.
func (apiSuite *APISuite) PatchV1Alpha1Config(provider config.Provider, patch func(*v1alpha1.Config)) []byte {
	ctr, err := provider.PatchV1Alpha1(func(c *v1alpha1.Config) error {
		patch(c)

		return nil
	})
	apiSuite.Require().NoError(err)

	bytes, err := ctr.Bytes()
	apiSuite.Require().NoError(err)

	return bytes
}

// TearDownSuite closes Talos API client.
func (apiSuite *APISuite) TearDownSuite() {
	if apiSuite.Client != nil {
		apiSuite.Assert().NoError(apiSuite.Client.Close())
	}
}

func mapNodeInfosToInternalIPs(nodes []cluster.NodeInfo) []string {
	ips := make([]string, len(nodes))

	for i, node := range nodes {
		ips[i] = node.InternalIP.String()
	}

	return ips
}
