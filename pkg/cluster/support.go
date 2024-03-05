// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"

	criconstants "github.com/containerd/containerd/pkg/cri/constants"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/formatters"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// BundleOptions defines GetSupportBundle options.
type BundleOptions struct {
	LogOutput  io.Writer
	NumWorkers int
	Source     string
	Client     *client.Client
	Archive    *BundleArchive
	Progress   chan BundleProgress

	lastLogMu sync.RWMutex
	lastLog   string
}

// BundleProgress reports current bundle collection progress.
type BundleProgress struct {
	Source string
	State  string
	Total  int
	Value  int
	Error  error
}

// BundleArchive wraps archive writer in a thread safe implementation.
type BundleArchive struct {
	Archive   *zip.Writer
	archiveMu sync.Mutex
}

// BundleError wraps all bundle collection errors and adds source context.
type BundleError struct {
	Source string

	err error
}

func (b *BundleError) Error() string {
	return b.err.Error()
}

func wrap(options *BundleOptions, err error) error {
	return &BundleError{
		Source: options.Source,
		err:    err,
	}
}

// Write creates a file in the archive.
func (a *BundleArchive) Write(path string, contents []byte) error {
	a.archiveMu.Lock()
	defer a.archiveMu.Unlock()

	file, err := a.Archive.Create(path)
	if err != nil {
		return err
	}

	_, err = file.Write(contents)
	if err != nil {
		return err
	}

	return nil
}

// Log writes the line to logger or to stdout if no logger was provided.
func (options *BundleOptions) Log(line string, args ...interface{}) {
	options.lastLogMu.Lock()
	defer options.lastLogMu.Unlock()

	options.lastLog = fmt.Sprintf(line, args...)

	if options.LogOutput != nil {
		options.LogOutput.Write([]byte(fmt.Sprintf(line, args...))) //nolint:errcheck

		return
	}

	fmt.Printf(line+"\n", args...)
}

type collect func(ctx context.Context, options *BundleOptions) ([]byte, error)

type nodeCollector struct {
	filename string
	collect  collect
}

var nodeCollectors = []nodeCollector{
	{"dmesg.log", dmesg},
	{"controller-runtime.log", logs("controller-runtime", false)},
	{"dependencies.dot", dependencies},
	{"mounts", mounts},
	{"devices", devices},
	{"io", ioPressure},
	{"processes", processes},
	{"summary", summary},
}

// GetNodeSupportBundle writes all node information we can gather into a zip archive.
//
//nolint:gocyclo
func GetNodeSupportBundle(ctx context.Context, options *BundleOptions) error {
	var errors error

	cols := nodeCollectors

	for _, dynamic := range []struct {
		id             string
		nodeCollectors func(context.Context, *client.Client) ([]nodeCollector, error)
	}{
		{"system services logs", getServiceLogCollectors},
		{"kube-system containers logs", getKubernetesLogCollectors},
		{"talos resources", getResources},
	} {
		var (
			dynamicCollectors []nodeCollector
			err               error
		)

		dynamicCollectors, err = dynamic.nodeCollectors(ctx, options.Client)
		if err != nil {
			errors = multierror.Append(errors, wrap(options, fmt.Errorf("failed to get %s %w", dynamic.id, err)))

			continue
		}

		cols = append(cols, dynamicCollectors...)
	}

	var eg errgroup.Group

	colChan := make(chan nodeCollector)

	eg.Go(func() error {
		defer func() {
			close(colChan)
		}()

		done := 0

		for _, nodeCollector := range cols {
			select {
			case colChan <- nodeCollector:
			case <-ctx.Done():
				return nil
			}

			done++

			options.lastLogMu.RLock()
			line := options.lastLog
			options.lastLogMu.RUnlock()

			if options.Progress != nil {
				options.Progress <- BundleProgress{Source: options.Source, Value: done, Total: len(cols), State: strings.Split(line, "\n")[0]}
			}
		}

		return nil
	})

	numWorkers := options.NumWorkers

	if len(cols) < options.NumWorkers {
		numWorkers = len(cols)
	}

	for i := 0; i < numWorkers; i++ {
		eg.Go(func() error {
			var errs error

			for nodeCollector := range colChan {
				if err := runCollector(ctx, nodeCollector, options); err != nil {
					errs = multierror.Append(errs, wrap(options, err))
				}
			}

			return errs
		})
	}

	if err := eg.Wait(); err != nil {
		return multierror.Append(errors, wrap(options, err))
	}

	return nil
}

// GetKubernetesSupportBundle writes cluster wide kubernetes information into a zip archive.
//
//nolint:gocyclo
func GetKubernetesSupportBundle(ctx context.Context, options *BundleOptions) error {
	var clientset *kubernetes.Clientset

	options.Source = "cluster"

	var errors error

	for _, node := range options.Client.GetEndpoints() {
		err := func() error {
			kubeconfig, err := options.Client.Kubeconfig(client.WithNodes(ctx, node))
			if err != nil {
				return err
			}

			config, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
			if err != nil {
				return err
			}

			restconfig, err := config.ClientConfig()
			if err != nil {
				return err
			}

			clientset, err = kubernetes.NewForConfig(restconfig)
			if err != nil {
				return err
			}

			// just checking that k8s responds
			_, err = clientset.CoreV1().Namespaces().Get(ctx, "kube-system", v1.GetOptions{})

			return err
		}()
		if err != nil {
			errors = multierror.Append(errors, wrap(options, err))

			continue
		}

		break
	}

	if clientset == nil {
		return multierror.Append(errors, wrap(
			options, fmt.Errorf("failed to get kubernetes client, tried nodes %s", strings.Join(options.Client.GetEndpoints(), ", "))),
		)
	}

	cols := []nodeCollector{
		{
			filename: "kubernetesResources/nodes.yaml",
			collect:  kubernetesNodes(clientset),
		},
		{
			filename: "kubernetesResources/systemPods.yaml",
			collect:  systemPods(clientset),
		},
	}

	for i, collector := range cols {
		if err := runCollector(ctx, collector, options); err != nil {
			errors = multierror.Append(errors, wrap(options, err))

			continue
		}

		if options.Progress != nil {
			options.Progress <- BundleProgress{Source: options.Source, Value: i, Total: len(cols), State: strings.Split(options.lastLog, "\n")[0]}
		}
	}

	return errors
}

func runCollector(ctx context.Context, c nodeCollector, options *BundleOptions) error {
	var (
		data []byte
		err  error
	)

	if data, err = c.collect(ctx, options); err != nil {
		return fmt.Errorf("failed to get %s: %s, skipped", c.filename, err)
	}

	if data == nil {
		return nil
	}

	return options.Archive.Write(fmt.Sprintf("%s/%s", options.Source, c.filename), data)
}

func getServiceLogCollectors(ctx context.Context, c *client.Client) ([]nodeCollector, error) {
	resp, err := c.ServiceList(ctx)
	if err != nil {
		return nil, err
	}

	cols := []nodeCollector{}

	for _, msg := range resp.Messages {
		for _, s := range msg.Services {
			cols = append(
				cols,
				nodeCollector{
					filename: fmt.Sprintf("%s.log", s.Id),
					collect:  logs(s.Id, false),
				},
				nodeCollector{
					filename: fmt.Sprintf("%s.state", s.Id),
					collect:  serviceInfo(s.Id),
				},
			)
		}
	}

	return cols, nil
}

func getKubernetesLogCollectors(ctx context.Context, c *client.Client) ([]nodeCollector, error) {
	namespace := criconstants.K8sContainerdNamespace
	driver := common.ContainerDriver_CRI

	resp, err := c.Containers(ctx, namespace, driver)
	if err != nil {
		return nil, err
	}

	cols := []nodeCollector{}

	for _, msg := range resp.Messages {
		for _, container := range msg.Containers {
			parts := strings.Split(container.PodId, "/")

			// skip pause containers
			if container.Status == "SANDBOX_READY" {
				continue
			}

			exited := ""

			if container.Pid == 0 {
				exited = "-exited"
			}

			if parts[0] == "kube-system" {
				cols = append(
					cols,
					nodeCollector{
						filename: fmt.Sprintf("%s/%s%s.log", parts[0], container.Name, exited),
						collect:  logs(container.Id, true),
					},
				)
			}
		}
	}

	return cols, err
}

func getResources(ctx context.Context, c *client.Client) ([]nodeCollector, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	nodes := md["nodes"]

	if len(nodes) != 1 {
		return nil, fmt.Errorf("got more than one node in the context: %v", nodes)
	}

	rds, err := safe.StateListAll[*meta.ResourceDefinition](client.WithNode(ctx, nodes[0]), c.COSI)
	if err != nil {
		return nil, err
	}

	it := rds.Iterator()

	cols := []nodeCollector{}

	for it.Next() {
		cols = append(cols, nodeCollector{
			filename: fmt.Sprintf("talosResources/%s.yaml", it.Value().Metadata().ID()),
			collect:  talosResource(it.Value()),
		})
	}

	return cols, nil
}

func serviceInfo(id string) collect {
	return func(ctx context.Context, options *BundleOptions) ([]byte, error) {
		services, err := options.Client.ServiceInfo(ctx, id)
		if err != nil {
			if services == nil {
				return nil, fmt.Errorf("error listing services: %w", err)
			}
		}

		var buf bytes.Buffer

		if err := formatters.RenderServicesInfo(services, &buf, "", false); err != nil {
			return nil, err
		}

		return buf.Bytes(), nil
	}
}

func dmesg(ctx context.Context, options *BundleOptions) ([]byte, error) {
	stream, err := options.Client.Dmesg(ctx, false, false)
	if err != nil {
		return nil, err
	}

	data := []byte{}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF || client.StatusCode(err) == codes.Canceled {
				break
			}

			return nil, fmt.Errorf("error reading from stream: %w", err)
		}

		if resp.Metadata != nil {
			if resp.Metadata.Error != "" {
				fmt.Fprintf(os.Stderr, "%s\n", resp.Metadata.Error)
			}
		}

		data = append(data, resp.GetBytes()...)
	}

	return data, nil
}

func logs(service string, kubernetes bool) collect {
	return func(ctx context.Context, options *BundleOptions) ([]byte, error) {
		var (
			namespace string
			driver    common.ContainerDriver
			err       error
		)

		if kubernetes {
			namespace = criconstants.K8sContainerdNamespace
			driver = common.ContainerDriver_CRI
		} else {
			namespace = constants.SystemContainerdNamespace
			driver = common.ContainerDriver_CONTAINERD
		}

		options.Log("getting %s/%s service logs", namespace, service)

		stream, err := options.Client.Logs(ctx, namespace, driver, service, false, -1)
		if err != nil {
			return nil, err
		}

		data := []byte{}

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF || client.StatusCode(err) == codes.Canceled {
					break
				}

				return nil, fmt.Errorf("error reading from stream: %w", err)
			}

			if resp.Metadata != nil {
				if resp.Metadata.Error != "" {
					fmt.Fprintf(os.Stderr, "%s\n", resp.Metadata.Error)
				}
			}

			data = append(data, resp.GetBytes()...)
		}

		return data, nil
	}
}

func dependencies(ctx context.Context, options *BundleOptions) ([]byte, error) {
	options.Log("inspecting controller runtime")

	resp, err := options.Client.Inspect.ControllerRuntimeDependencies(ctx)
	if err != nil {
		if resp == nil {
			return nil, fmt.Errorf("error getting controller runtime dependencies: %s", err)
		}
	}

	var buf bytes.Buffer

	if err = formatters.RenderGraph(ctx, options.Client, resp, &buf, true); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func talosResource(rd *meta.ResourceDefinition) collect {
	return func(ctx context.Context, options *BundleOptions) ([]byte, error) {
		options.Log("getting talos resource %s/%s", rd.TypedSpec().DefaultNamespace, rd.TypedSpec().Type)

		resources, err := listResources(ctx, options.Client, rd.TypedSpec().DefaultNamespace, rd.TypedSpec().Type)
		if err != nil {
			return nil, err
		}

		var (
			buf      bytes.Buffer
			hasItems bool
		)

		encoder := yaml.NewEncoder(&buf)

		for _, r := range resources {
			data := struct {
				Metadata *resource.Metadata `yaml:"metadata"`
				Spec     interface{}        `yaml:"spec"`
			}{
				Metadata: r.Metadata(),
				Spec:     "<REDACTED>",
			}

			if rd.TypedSpec().Sensitivity != meta.Sensitive {
				data.Spec = r.Spec()
			}

			if err = encoder.Encode(&data); err != nil {
				return nil, err
			}

			hasItems = true
		}

		if !hasItems {
			return nil, nil
		}

		return buf.Bytes(), encoder.Close()
	}
}

func kubernetesNodes(client *kubernetes.Clientset) collect {
	return func(ctx context.Context, options *BundleOptions) ([]byte, error) {
		options.Log("getting kubernetes nodes manifests")

		nodes, err := client.CoreV1().Nodes().List(ctx, v1.ListOptions{})
		if err != nil {
			return nil, err
		}

		return marshalYAML(nodes)
	}
}

func systemPods(client *kubernetes.Clientset) collect {
	return func(ctx context.Context, options *BundleOptions) ([]byte, error) {
		options.Log("getting pods manifests in kube-system namespace")

		nodes, err := client.CoreV1().Pods("kube-system").List(ctx, v1.ListOptions{})
		if err != nil {
			return nil, err
		}

		return marshalYAML(nodes)
	}
}

func mounts(ctx context.Context, options *BundleOptions) ([]byte, error) {
	options.Log("getting mounts")

	resp, err := options.Client.Mounts(ctx)
	if err != nil {
		if resp == nil {
			return nil, fmt.Errorf("error getting interfaces: %s", err)
		}
	}

	var buf bytes.Buffer

	if err = formatters.RenderMounts(resp, &buf, nil); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func devices(ctx context.Context, options *BundleOptions) ([]byte, error) {
	options.Log("reading devices")

	r, err := options.Client.Read(ctx, "/proc/bus/pci/devices")
	if err != nil {
		return nil, err
	}

	defer r.Close() //nolint:errcheck

	return io.ReadAll(r)
}

func ioPressure(ctx context.Context, options *BundleOptions) ([]byte, error) {
	options.Log("getting disk stats")

	resp, err := options.Client.MachineClient.DiskStats(ctx, &emptypb.Empty{})

	var filtered interface{}
	filtered, err = client.FilterMessages(resp, err)
	resp, _ = filtered.(*machine.DiskStatsResponse) //nolint:errcheck

	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tIO_TIME\tIO_TIME_WEIGHTED\tDISK_WRITE_SECTORS\tDISK_READ_SECTORS")

	for _, msg := range resp.Messages {
		for _, stat := range msg.Devices {
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n",
				stat.Name,
				stat.IoTimeMs,
				stat.IoTimeWeightedMs,
				stat.WriteSectors,
				stat.ReadSectors,
			)
		}
	}

	if err = w.Flush(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func processes(ctx context.Context, options *BundleOptions) ([]byte, error) {
	options.Log("getting processes snapshot")

	resp, err := options.Client.Processes(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PID\tSTATE\tTHREADS\tCPU-TIME\tVIRTMEM\tRESMEM\tCOMMAND")

	for _, msg := range resp.Messages {
		procs := msg.Processes

		var args string

		for _, p := range procs {
			switch {
			case p.Executable == "":
				args = p.Command
			case p.Args != "" && strings.Fields(p.Args)[0] == filepath.Base(strings.Fields(p.Executable)[0]):
				args = strings.Replace(p.Args, strings.Fields(p.Args)[0], p.Executable, 1)
			default:
				args = p.Args
			}

			fmt.Fprintf(w, "%6d\t%1s\t%4d\t%8.2f\t%7s\t%7s\t%s\n",
				p.Pid, p.State, p.Threads, p.CpuTime, humanize.Bytes(p.VirtualMemory), humanize.Bytes(p.ResidentMemory), args)
		}
	}

	if err := w.Flush(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func summary(ctx context.Context, options *BundleOptions) ([]byte, error) {
	var buf bytes.Buffer

	fmt.Fprintln(&buf, "Client:")
	version.WriteLongVersionFromExisting(&buf, version.NewVersion())

	resp, err := options.Client.Version(ctx)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(&buf, "Server:")

	for _, m := range resp.Messages {
		version.WriteLongVersionFromExisting(&buf, m.Version)
	}

	return buf.Bytes(), nil
}

func listResources(ctx context.Context, c *client.Client, namespace, resourceType string) ([]resource.Resource, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	nodes := md["nodes"]

	if len(nodes) != 1 {
		return nil, fmt.Errorf("got more than one node in the context: %v", nodes)
	}

	items, err := c.COSI.List(client.WithNode(ctx, nodes[0]), resource.NewMetadata(namespace, resourceType, "", resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	return items.Items, nil
}

func marshalYAML(resource runtime.Object) ([]byte, error) {
	serializer := k8sjson.NewSerializerWithOptions(
		k8sjson.DefaultMetaFactory, nil, nil,
		k8sjson.SerializerOptions{
			Yaml:   true,
			Pretty: true,
			Strict: true,
		},
	)

	var buf bytes.Buffer

	if err := serializer.Encode(resource, &buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
