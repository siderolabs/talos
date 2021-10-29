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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/dustin/go-humanize"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/version"
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

	errorsMu sync.Mutex
	Errors   []error
}

// BundleProgress reports current bundle collection progress.
type BundleProgress struct {
	Source string
	State  string
	Total  int
	Value  int
}

// BundleArchive wraps archive writer in a thread safe implementation.
type BundleArchive struct {
	Archive   *zip.Writer
	archiveMu sync.Mutex
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

// Error logs error.
func (options *BundleOptions) Error(err error) {
	options.errorsMu.Lock()
	defer options.errorsMu.Unlock()

	options.Errors = append(options.Errors, err)
}

// Errorf logs error.
func (options *BundleOptions) Errorf(format string, args ...interface{}) {
	options.Error(fmt.Errorf(format, args...))
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
//nolint:gocyclo
func GetNodeSupportBundle(ctx context.Context, options *BundleOptions) error {
	var err error

	cols := nodeCollectors

	for _, dynamic := range []struct {
		id             string
		nodeCollectors func(context.Context, *client.Client) ([]nodeCollector, error)
	}{
		{"system services logs", getServiceLogCollectors},
		{"kube-system containers logs", getKubernetesLogCollectors},
		{"talos resources", getResources},
	} {
		var dynamicCollectors []nodeCollector

		dynamicCollectors, err = dynamic.nodeCollectors(ctx, options.Client)
		if err != nil {
			options.Errorf("failed to get %s %w", dynamic.id, err)

			continue
		}

		cols = append(cols, dynamicCollectors...)
	}

	var wg sync.WaitGroup

	wg.Add(1)

	colChan := make(chan nodeCollector)

	go func() {
		defer func() {
			close(colChan)
			wg.Done()
		}()

		done := 0

		for _, nodeCollector := range cols {
			select {
			case colChan <- nodeCollector:
			case <-ctx.Done():
				return
			}

			done++

			options.lastLogMu.RLock()
			line := options.lastLog
			options.lastLogMu.RUnlock()

			if options.Progress != nil {
				options.Progress <- BundleProgress{Source: options.Source, Value: done, Total: len(cols), State: strings.Split(line, "\n")[0]}
			}
		}
	}()

	numWorkers := options.NumWorkers

	if len(cols) < options.NumWorkers {
		numWorkers = len(cols)
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for nodeCollector := range colChan {
				if runCollector(ctx, nodeCollector, options) != nil {
					return
				}
			}
		}()
	}

	wg.Wait()

	return nil
}

// GetKubernetesSupportBundle writes cluster wide kubernetes information into a zip archive.
//nolint:gocyclo
func GetKubernetesSupportBundle(ctx context.Context, options *BundleOptions) error {
	var clientset *kubernetes.Clientset

	options.Source = "cluster"

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
			options.Error(err)

			continue
		}

		break
	}

	if clientset == nil {
		return fmt.Errorf("failed to get kubernetes client client, tried nodes %s", strings.Join(options.Client.GetEndpoints(), ", "))
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
			return err
		}

		if options.Progress != nil {
			options.Progress <- BundleProgress{Source: options.Source, Value: i, Total: len(cols), State: strings.Split(options.lastLog, "\n")[0]}
		}
	}

	return nil
}

func runCollector(ctx context.Context, c nodeCollector, options *BundleOptions) error {
	var (
		data []byte
		err  error
	)

	if data, err = c.collect(ctx, options); err != nil {
		options.Errorf("failed to get %s: %s, skipped", c.filename, err)

		return nil
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
	responses, err := listResources(ctx, c, meta.NamespaceName, meta.ResourceDefinitionType)
	if err != nil {
		return nil, err
	}

	cols := []nodeCollector{}

	for _, msg := range responses {
		if msg.Resource == nil {
			continue
		}

		b, err := yaml.Marshal(msg.Resource.Spec())
		if err != nil {
			return nil, err
		}

		spec := &meta.ResourceDefinitionSpec{}

		if err = yaml.Unmarshal(b, spec); err != nil {
			return nil, err
		}

		cols = append(cols, nodeCollector{
			filename: fmt.Sprintf("talosResources/%s.yaml", spec.ID()),
			collect:  talosResource(spec),
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

			options.Error(err)
		}

		var buf bytes.Buffer

		if err := cli.RenderServicesInfo(services, &buf, "", false); err != nil {
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

	if err = cli.RenderGraph(ctx, options.Client, resp, &buf, true); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func talosResource(rd *meta.ResourceDefinitionSpec) collect {
	return func(ctx context.Context, options *BundleOptions) ([]byte, error) {
		options.Log("getting talos resource %s/%s", rd.DefaultNamespace, rd.ID())

		responses, err := listResources(ctx, options.Client, rd.DefaultNamespace, rd.ID())
		if err != nil {
			return nil, err
		}

		var (
			buf      bytes.Buffer
			hasItems bool
		)

		encoder := yaml.NewEncoder(&buf)

		for _, msg := range responses {
			if msg.Resource == nil {
				continue
			}

			data := struct {
				Metadata *resource.Metadata `yaml:"metadata"`
				Spec     interface{}        `yaml:"spec"`
			}{
				Metadata: msg.Resource.Metadata(),
				Spec:     "<REDACTED>",
			}

			if rd.Sensitivity != meta.Sensitive {
				data.Spec = msg.Resource.Spec()
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

	if err = cli.RenderMounts(resp, &buf, nil); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func devices(ctx context.Context, options *BundleOptions) ([]byte, error) {
	options.Log("reading devices")

	r, _, err := options.Client.Read(ctx, "/proc/bus/pci/devices")
	if err != nil {
		return nil, err
	}

	defer r.Close() //nolint:errcheck

	return ioutil.ReadAll(r)
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
	resp, err := options.Client.Version(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	for _, m := range resp.Messages {
		version.WriteLongVersionFromExisting(&buf, m.Version)
	}

	return buf.Bytes(), nil
}

func listResources(ctx context.Context, c *client.Client, namespace, resourceType string) ([]client.ResourceResponse, error) {
	listClient, err := c.Resources.List(ctx, namespace, resourceType)
	if err != nil {
		return nil, err
	}

	resources := []client.ResourceResponse{}

	for {
		msg, err := listClient.Recv()
		if err != nil {
			if err == io.EOF || client.StatusCode(err) == codes.Canceled {
				return resources, nil
			}

			return nil, err
		}

		if msg.Metadata.GetError() != "" {
			fmt.Fprintf(os.Stderr, "%s: %s\n", msg.Metadata.GetHostname(), msg.Metadata.GetError())

			continue
		}

		resources = append(resources, msg)
	}
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
