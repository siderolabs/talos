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
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/xslices"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
	"github.com/siderolabs/talos/internal/pkg/dashboard/logdata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

func init() {
	// set background to be left as the default color of the terminal
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
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

	// ScreenResourceExplorer is the resource explorer screen.
	ScreenResourceExplorer Screen = "Resources"
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

// TickerListener is a listener which is notified on every tick.
type TickerListener interface {
	OnTick()
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
	tickerListeners       []TickerListener

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
	nodes             []string
}

// buildDashboard initializes the summary dashboard.
//
//nolint:gocyclo,cyclop
func buildDashboard(ctx context.Context, cli *client.Client, opts ...Option) (*Dashboard, error) {
	options := defaultOptions()

	for _, opt := range opts {
		opt(options)
	}

	nodes := getSortedNodeAliases(options.nodes)

	dashboard := &Dashboard{
		cli:      cli,
		interval: options.interval,
		app:      tview.NewApplication(),
		nodeSet:  make(map[string]struct{}),
		nodes:    nodes,
	}

	dashboard.mainGrid = tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(0)

	dashboard.pages = tview.NewPages().AddPage(pageMain, dashboard.mainGrid, true, true)

	dashboard.app.EnableMouse(true)
	dashboard.app.SetRoot(dashboard.pages, true).SetFocus(dashboard.pages)

	header := components.NewHeader()
	dashboard.mainGrid.AddItem(header, 0, 0, 1, 1, 0, 0, false)

	if err := dashboard.initScreenConfigs(ctx, options.screens); err != nil {
		return nil, err
	}

	screenKeyToName := xslices.ToMap(dashboard.screenConfigs, func(t screenConfig) (string, string) {
		return t.screenKey, string(t.screen)
	})

	screenConfigByKeyCode := xslices.ToMap(dashboard.screenConfigs, func(config screenConfig) (tcell.Key, screenConfig) {
		return config.keyCode, config
	})

	dashboard.footer = components.NewFooter(screenKeyToName, nodes)

	dashboard.footer.NodeClick = func(node string) {
		allowNodeNavigation := dashboard.selectedScreenConfig != nil && dashboard.selectedScreenConfig.allowNodeNavigation
		if !allowNodeNavigation {
			return
		}

		for i, n := range dashboard.nodes {
			if n == node {
				dashboard.selectNodeByIndex(i)

				break
			}
		}
	}

	dashboard.footer.ScreenClick = func(screenName string) {
		dashboard.selectScreen(Screen(screenName))
	}

	dashboard.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		config, screenOk := screenConfigByKeyCode[event.Key()]

		allowNodeNavigation := dashboard.selectedScreenConfig != nil && dashboard.selectedScreenConfig.allowNodeNavigation

		// When a text input field has focus, pass all printable-character keys
		// and navigation keys through so the field can consume them. Only global
		// shortcuts (Ctrl+Z, Ctrl+C, function-key screen switches) remain active.
		_, focusedIsInput := dashboard.app.GetFocus().(*tview.InputField)

		switch {
		case screenOk:
			dashboard.selectScreen(config.screen)

			return nil
		case !focusedIsInput && allowNodeNavigation && (event.Key() == tcell.KeyLeft || event.Rune() == 'h'):
			dashboard.selectNodeByIndex(dashboard.selectedNodeIndex - 1)

			return nil
		case !focusedIsInput && allowNodeNavigation && (event.Key() == tcell.KeyRight || event.Rune() == 'l'):
			dashboard.selectNodeByIndex(dashboard.selectedNodeIndex + 1)

			return nil
		case !focusedIsInput && options.allowExitKeys && (event.Key() == tcell.KeyCtrlC || event.Rune() == 'q'):
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

	dashboard.tickerListeners = []TickerListener{
		header,
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

	dashboard.apiDataSource = &apidata.Source{
		Client:   cli,
		Interval: options.interval,
		Nodes:    nodes,
	}

	dashboard.resourceDataSource = &resourcedata.Source{
		COSI:  cli.COSI,
		Nodes: nodes,
	}

	dashboard.logDataSource = logdata.NewSource(cli, nodes)

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
		case ScreenResourceExplorer:
			return NewResourceExplorerGrid(ctx, d)
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
//
//nolint:gocyclo
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

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case nodeLog := <-d.logDataSource.LogCh:
				// Drain any additional log lines that are immediately available,
				// so a burst of logs produces one closure instead of one per line.
				logs := []logdata.Data{nodeLog}

			drainLogs:
				for {
					select {
					case nl := <-d.logDataSource.LogCh:
						logs = append(logs, nl)
					default:
						break drainLogs
					}
				}

				d.app.QueueUpdate(func() {
					for _, l := range logs {
						d.processLog(l.Node, l.Log, l.Error)
					}
				})
			case d.data = <-dataCh:
				d.app.QueueUpdate(func() {
					if !d.paused {
						d.processAPIData()
					}
				})
			case nodeResource := <-d.resourceDataSource.NodeResourceCh:
				d.app.QueueUpdate(func() {
					d.processNodeResource(nodeResource)
				})
			case <-ticker.C:
				// Only the ticker triggers a full redraw, capping redraws at 2 fps.
				d.app.QueueUpdateDraw(func() {
					if !d.paused {
						d.processTick()
					}
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

// processTick re-renders the components with ticker.
func (d *Dashboard) processTick() {
	for _, component := range d.tickerListeners {
		component.OnTick()
	}
}

func (d *Dashboard) selectScreen(screen Screen) {
	for _, info := range d.screenConfigs {
		if info.screen == screen {
			d.selectedScreenConfig = &info

			d.mainGrid.AddItem(info.primitive, 1, 0, 1, 1, 0, 0, false)

			info.primitive.onScreenSelect(true)

			continue
		}

		d.mainGrid.RemoveItem(info.primitive)
		info.primitive.onScreenSelect(false)
	}

	d.footer.SelectScreen(string(screen))
}

// getSortedNodeAliases returns the unique node aliases sorted by their IP address.
func getSortedNodeAliases(nodeAliases []string) []string {
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
