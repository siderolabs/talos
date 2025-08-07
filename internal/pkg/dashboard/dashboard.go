// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dashboard implements a text-based UI dashboard.
package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	_ "github.com/gdamore/tcell/v2/terminfo/l/linux" // linux terminal is used when running on the machine, but not included with tcell_minimal
	"github.com/gizak/termui/v3"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
	"github.com/siderolabs/talos/internal/pkg/dashboard/logdata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/resolver"
	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

func init() {
	// set background to be left as the default color of the terminal
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault

	// set the titles of the termui (legacy) to be bold
	termui.Theme.Block.Title.Modifier = termui.ModifierBold
}

// Screen is a dashboard screen.
type Screen string

const (
	pageMain = "main"

	// ScreenSummary is the summary screen.
	ScreenSummary Screen = "Summary"

	// ScreenMonitor is the monitor (metrics) screen.
	ScreenMonitor Screen = "Monitor"

	// ScreenNetworkConfig is the network configuration screen.
	ScreenNetworkConfig Screen = "Network Config"

	// ScreenConfigURL is the config URL screen.
	ScreenConfigURL Screen = "Config URL"
)

// APIDataListener is a listener which is notified when API-sourced data is updated.
type APIDataListener interface {
	OnAPIDataChange(node string, data *apidata.Data)
}

// ResourceDataListener is a listener which is notified when a resource is updated.
type ResourceDataListener interface {
	OnResourceDataChange(data resourcedata.Data)
}

// LogDataListener is a listener which is notified when a log line is received.
type LogDataListener interface {
	OnLogDataChange(node, logLine, logError string)
}

// NodeSelectListener is a listener which is notified when a node is selected.
type NodeSelectListener interface {
	OnNodeSelect(node string)
}

type screenConfig struct {
	screenKey           string
	screen              Screen
	keyCode             tcell.Key
	primitive           screenSelectListener
	allowNodeNavigation bool
}

// screenSelectListener is a listener which is notified when a screen is selected.
type screenSelectListener interface {
	tview.Primitive

	onScreenSelect(active bool)
}

// Dashboard implements the summary dashboard.
type Dashboard struct {
	cli      *client.Client
	interval time.Duration

	apiDataSource      *apidata.Source
	resourceDataSource *resourcedata.Source
	logDataSource      *logdata.Source

	apiDataListeners      []APIDataListener
	resourceDataListeners []ResourceDataListener
	logDataListeners      []LogDataListener
	nodeSelectListeners   []NodeSelectListener

	app *tview.Application

	mainGrid *tview.Grid

	pages *tview.Pages

	selectedScreenConfig *screenConfig
	screenConfigs        []screenConfig
	footer               *components.Footer

	data *apidata.Data

	selectedNodeIndex int
	selectedNode      string
	paused            bool
	nodeSet           map[string]struct{}
	ipsToNodeAliases  map[string]string
	nodes             []string
}

// buildDashboard initializes the summary dashboard.
//
//nolint:gocyclo,cyclop
func buildDashboard(ctx context.Context, cli *client.Client, opts ...Option) (*Dashboard, error) {
	defOptions := defaultOptions()

	for _, opt := range opts {
		opt(defOptions)
	}

	// map node IPs to their aliases (names/IPs - as specified "nodes" in context).
	// this will also trigger the interactive API authentication if needed - e.g., when the API is used through Omni.
	ipsToNodeAliases, err := collectNodeIPsToNodeAliases(ctx, cli)
	if err != nil {
		return nil, err
	}

	nodes := getSortedNodeAliases(ipsToNodeAliases)

	dashboard := &Dashboard{
		cli:              cli,
		interval:         defOptions.interval,
		app:              tview.NewApplication(),
		nodeSet:          make(map[string]struct{}),
		nodes:            nodes,
		ipsToNodeAliases: ipsToNodeAliases,
	}

	dashboard.mainGrid = tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(0)

	dashboard.pages = tview.NewPages().AddPage(pageMain, dashboard.mainGrid, true, true)

	dashboard.app.SetRoot(dashboard.pages, true).SetFocus(dashboard.pages)

	header := components.NewHeader()
	dashboard.mainGrid.AddItem(header, 0, 0, 1, 1, 0, 0, false)

	if err = dashboard.initScreenConfigs(ctx, defOptions.screens); err != nil {
		return nil, err
	}

	screenKeyToName := xslices.ToMap(dashboard.screenConfigs, func(t screenConfig) (string, string) {
		return t.screenKey, string(t.screen)
	})

	screenConfigByKeyCode := xslices.ToMap(dashboard.screenConfigs, func(config screenConfig) (tcell.Key, screenConfig) {
		return config.keyCode, config
	})

	dashboard.footer = components.NewFooter(screenKeyToName, nodes)

	dashboard.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		config, screenOk := screenConfigByKeyCode[event.Key()]

		allowNodeNavigation := dashboard.selectedScreenConfig != nil && dashboard.selectedScreenConfig.allowNodeNavigation

		switch {
		case screenOk:
			dashboard.selectScreen(config.screen)

			return nil
		case allowNodeNavigation && (event.Key() == tcell.KeyLeft || event.Rune() == 'h'):
			dashboard.selectNodeByIndex(dashboard.selectedNodeIndex - 1)

			return nil
		case allowNodeNavigation && (event.Key() == tcell.KeyRight || event.Rune() == 'l'):
			dashboard.selectNodeByIndex(dashboard.selectedNodeIndex + 1)

			return nil
		case defOptions.allowExitKeys && (event.Key() == tcell.KeyCtrlC || event.Rune() == 'q'):
			dashboard.app.Stop()

			return nil
		case event.Key() == tcell.KeyCtrlZ:
			dashboard.paused = !dashboard.paused
			dashboard.footer.SetPaused(dashboard.paused)

			return nil
		}

		return event
	})

	dashboard.mainGrid.AddItem(dashboard.footer, 2, 0, 1, 1, 0, 0, false)

	dashboard.apiDataListeners = []APIDataListener{
		header,
	}

	dashboard.resourceDataListeners = []ResourceDataListener{
		header,
	}

	dashboard.logDataListeners = []LogDataListener{}

	dashboard.nodeSelectListeners = []NodeSelectListener{
		header,
		dashboard.footer,
	}

	for _, config := range dashboard.screenConfigs {
		screenPrimitive := config.primitive

		apiDataListener, ok := screenPrimitive.(APIDataListener)
		if ok {
			dashboard.apiDataListeners = append(dashboard.apiDataListeners, apiDataListener)
		}

		resourceDataListener, ok := screenPrimitive.(ResourceDataListener)
		if ok {
			dashboard.resourceDataListeners = append(dashboard.resourceDataListeners, resourceDataListener)
		}

		logDataListener, ok := screenPrimitive.(LogDataListener)
		if ok {
			dashboard.logDataListeners = append(dashboard.logDataListeners, logDataListener)
		}

		nodeSelectListener, ok := screenPrimitive.(NodeSelectListener)
		if ok {
			dashboard.nodeSelectListeners = append(dashboard.nodeSelectListeners, nodeSelectListener)
		}
	}

	nodeResolver := resolver.New(ipsToNodeAliases)

	dashboard.apiDataSource = &apidata.Source{
		Client:   cli,
		Interval: defOptions.interval,
		Resolver: nodeResolver,
	}

	dashboard.resourceDataSource = &resourcedata.Source{
		COSI: cli.COSI,
	}

	dashboard.logDataSource = logdata.NewSource(cli, nodeResolver)

	return dashboard, nil
}

func (d *Dashboard) initScreenConfigs(ctx context.Context, screens []Screen) error {
	primitiveForScreen := func(screen Screen) screenSelectListener {
		switch screen {
		case ScreenSummary:
			return NewSummaryGrid(d.app)
		case ScreenMonitor:
			return NewMonitorGrid(d.app)
		case ScreenNetworkConfig:
			return NewNetworkConfigGrid(ctx, d)
		case ScreenConfigURL:
			return NewConfigURLGrid(ctx, d)
		default:
			return nil
		}
	}

	d.screenConfigs = make([]screenConfig, 0, len(screens))

	for i, screen := range screens {
		primitive := primitiveForScreen(screen)
		if primitive == nil {
			return fmt.Errorf("unknown screen %s", screen)
		}

		config := screenConfig{
			screenKey:           fmt.Sprintf("F%d", i+1),
			screen:              screen,
			keyCode:             tcell.KeyF1 + tcell.Key(i),
			primitive:           primitive,
			allowNodeNavigation: true,
		}

		if screen == ScreenNetworkConfig || screen == ScreenConfigURL {
			config.allowNodeNavigation = false
		}

		d.screenConfigs = append(d.screenConfigs, config)
	}

	return nil
}

// Run starts the dashboard.
func Run(ctx context.Context, cli *client.Client, opts ...Option) (runErr error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dashboard, err := buildDashboard(ctx, cli, opts...)
	if err != nil {
		return err
	}

	dashboard.selectNodeByIndex(0)

	// handle panic & stop dashboard gracefully on exit
	defer func() {
		if r := recover(); r != nil {
			runErr = fmt.Errorf("dashboard panic: %v", r)
		}

		dashboard.app.Stop()
	}()

	dashboard.selectScreen(ScreenSummary)

	eg, ctx := errgroup.WithContext(ctx)

	stopFunc := dashboard.startDataHandler(ctx)
	defer stopFunc() //nolint:errcheck

	eg.Go(func() error {
		defer cancel()

		return dashboard.app.Run()
	})

	// stop dashboard when the context is canceled
	eg.Go(func() error {
		<-ctx.Done()

		dashboard.app.Stop()

		return nil
	})

	return eg.Wait()
}

// startDataHandler starts the data and log update handler and returns a function to stop it.
func (d *Dashboard) startDataHandler(ctx context.Context) func() error {
	var eg errgroup.Group

	ctx, cancel := context.WithCancel(ctx)

	stopFunc := func() error {
		cancel()

		err := eg.Wait()
		if errors.Is(err, context.Canceled) {
			return nil
		}

		return err
	}

	eg.Go(func() error {
		// start API data source
		dataCh := d.apiDataSource.Run(ctx)
		defer d.apiDataSource.Stop()

		// start resources data source
		d.resourceDataSource.Run(ctx)
		defer d.resourceDataSource.Stop() //nolint:errcheck

		// start logs data source
		d.logDataSource.Start(ctx)
		defer d.logDataSource.Stop() //nolint:errcheck

		lastLogTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case nodeLog := <-d.logDataSource.LogCh:
				if time.Since(lastLogTime) < 50*time.Millisecond {
					d.app.QueueUpdate(func() {
						d.processLog(nodeLog.Node, nodeLog.Log, nodeLog.Error)
					})
				} else {
					d.app.QueueUpdateDraw(func() {
						d.processLog(nodeLog.Node, nodeLog.Log, nodeLog.Error)
					})
				}

				lastLogTime = time.Now()
			case d.data = <-dataCh:
				d.app.QueueUpdateDraw(func() {
					if !d.paused {
						d.processAPIData()
					}
				})
			case nodeResource := <-d.resourceDataSource.NodeResourceCh:
				d.app.QueueUpdateDraw(func() {
					d.processNodeResource(nodeResource)
				})
			}
		}
	})

	return stopFunc
}

func (d *Dashboard) selectNodeByIndex(index int) {
	if len(d.nodes) == 0 {
		return
	}

	if index < 0 {
		index = 0
	} else if index >= len(d.nodes) {
		index = len(d.nodes) - 1
	}

	d.selectedNode = d.nodes[index]
	d.selectedNodeIndex = index

	d.processAPIData()

	for _, listener := range d.nodeSelectListeners {
		listener.OnNodeSelect(d.selectedNode)
	}
}

// processAPIData re-renders the components with new API-sourced data.
func (d *Dashboard) processAPIData() {
	if d.data == nil {
		return
	}

	for _, component := range d.apiDataListeners {
		component.OnAPIDataChange(d.selectedNode, d.data)
	}
}

// processNodeResource re-renders the components with new resource data.
func (d *Dashboard) processNodeResource(nodeResource resourcedata.Data) {
	for _, component := range d.resourceDataListeners {
		component.OnResourceDataChange(nodeResource)
	}
}

// processLog re-renders the log components with new log data.
func (d *Dashboard) processLog(node, logLine, logError string) {
	for _, component := range d.logDataListeners {
		component.OnLogDataChange(node, logLine, logError)
	}
}

func (d *Dashboard) selectScreen(screen Screen) {
	for _, info := range d.screenConfigs {
		if info.screen == screen {
			d.selectedScreenConfig = &info //nolint:exportloopref

			d.mainGrid.AddItem(info.primitive, 1, 0, 1, 1, 0, 0, false)

			info.primitive.onScreenSelect(true)

			continue
		}

		d.mainGrid.RemoveItem(info.primitive)
		info.primitive.onScreenSelect(false)
	}

	d.footer.SelectScreen(string(screen))
}

// collectNodeIPsToNodeAliases probes all nodes in the context for their IP addresses by calling their .Version endpoint and maps them to the node aliases in the context.
//
// Sample output:
//
// 172.20.0.6 -> node-1
//
// 10.42.0.1 -> node-1
//
// 172.20.0.7 -> node-2
//
// 10.42.0.2 -> node-2.
func collectNodeIPsToNodeAliases(ctx context.Context, c *client.Client) (map[string]string, error) {
	ipsToNodeAliases := make(map[string]string)

	nodes := nodeAliasesInContext(ctx)
	for _, node := range nodes {
		ctx = client.WithNodes(ctx, node) //nolint:fatcontext // do not replace this with "WithNode" - it would not return the IP in the response metadata

		resp, err := c.Version(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get node %q version: %w", node, err)
		}

		if len(resp.GetMessages()) == 0 {
			return nil, fmt.Errorf("node %q returned no messages in version response", node)
		}

		nodeIP := resp.GetMessages()[0].GetMetadata().GetHostname()
		if nodeIP == "" {
			return nil, fmt.Errorf("node %q returned no IP in version response", node)
		}

		ipsToNodeAliases[nodeIP] = node
	}

	return ipsToNodeAliases, nil
}

// nodeAliasesInContext extracts the node aliases (IP, name etc.) from the given context which are stored in the "node" or "nodes" GRPC metadata.
func nodeAliasesInContext(ctx context.Context) []string {
	md, mdOk := metadata.FromOutgoingContext(ctx)
	if !mdOk {
		return nil
	}

	nodeVal := md.Get("node")
	if len(nodeVal) > 0 {
		return []string{nodeVal[0]}
	}

	nodesVal := md.Get(message.NodesHeaderKey)

	return xslices.FlatMap(nodesVal, func(node string) []string {
		return strings.Split(node, ",")
	})
}

// getSortedNodeAliases returns the unique node aliases sorted by their IP address.
func getSortedNodeAliases(ipToNodeAliases map[string]string) []string {
	if len(ipToNodeAliases) == 0 { // assume that it is the local node (running on TTY)
		return []string{""}
	}

	nodeAliases := maps.Keys(xslices.ToSet(maps.Values(ipToNodeAliases))) // eliminate duplicates

	// if the aliases are IP addresses, compare them as IPs
	// otherwise, compare them as strings
	// all IPs come before non-IPs
	slices.SortFunc(nodeAliases, func(a, b string) int {
		addrA, aErr := netip.ParseAddr(a)

		addrB, bErr := netip.ParseAddr(b)
		if aErr != nil && bErr != nil {
			return strings.Compare(a, b)
		}

		if aErr != nil {
			return 1
		}

		if bErr != nil {
			return -1
		}

		return addrA.Compare(addrB)
	})

	return nodeAliases
}
