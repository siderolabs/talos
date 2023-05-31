// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dashboard implements a text-based UI dashboard.
package dashboard

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gizak/termui/v3"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/slices"
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
	OnLogDataChange(node string, logLine string)
}

// NodeSetListener is a listener which is notified when the set of nodes changes.
type NodeSetListener interface {
	OnNodeSetChange(nodes []string)
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

	apiDataListeners       []APIDataListener
	resourceDataListeners  []ResourceDataListener
	logDataListeners       []LogDataListener
	nodeSelectListeners    []NodeSelectListener
	nodeSetChangeListeners []NodeSetListener

	app *tview.Application

	mainGrid *tview.Grid

	pages *tview.Pages

	selectedScreenConfig *screenConfig
	screenConfigs        []screenConfig
	footer               *components.Footer

	data *apidata.Data

	selectedNodeIndex int
	selectedNode      string
	nodeSet           map[string]struct{}
	nodes             []string
}

// buildDashboard initializes the summary dashboard.
//
//nolint:gocyclo
func buildDashboard(ctx context.Context, cli *client.Client, opts ...Option) (*Dashboard, error) {
	defOptions := defaultOptions()

	for _, opt := range opts {
		opt(defOptions)
	}

	dashboard := &Dashboard{
		cli:      cli,
		interval: defOptions.interval,
		app:      tview.NewApplication(),
		nodeSet:  make(map[string]struct{}),
	}

	dashboard.mainGrid = tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(0)

	dashboard.pages = tview.NewPages().AddPage(pageMain, dashboard.mainGrid, true, true)

	header := components.NewHeader()
	dashboard.mainGrid.AddItem(header, 0, 0, 1, 1, 0, 0, false)

	err := dashboard.initScreenConfigs(ctx, defOptions.screens)
	if err != nil {
		return nil, err
	}

	screenKeyToName := slices.ToMap(dashboard.screenConfigs, func(t screenConfig) (string, string) {
		return t.screenKey, string(t.screen)
	})

	screenConfigByKeyCode := slices.ToMap(dashboard.screenConfigs, func(config screenConfig) (tcell.Key, screenConfig) {
		return config.keyCode, config
	})

	dashboard.footer = components.NewFooter(screenKeyToName)

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

	dashboard.nodeSetChangeListeners = []NodeSetListener{
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

		nodeSetListener, ok := screenPrimitive.(NodeSetListener)
		if ok {
			dashboard.nodeSetChangeListeners = append(dashboard.nodeSetChangeListeners, nodeSetListener)
		}
	}

	dashboard.apiDataSource = &apidata.Source{
		Client:   cli,
		Interval: defOptions.interval,
	}

	dashboard.resourceDataSource = &resourcedata.Source{
		COSI: cli.COSI,
	}

	dashboard.logDataSource = logdata.NewSource(cli)

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
func Run(ctx context.Context, cli *client.Client, opts ...Option) error {
	dashboard, err := buildDashboard(ctx, cli, opts...)
	if err != nil {
		return err
	}

	dashboard.selectScreen(ScreenSummary)

	stopFunc := dashboard.startDataHandler(ctx)
	defer stopFunc() //nolint:errcheck

	if err = dashboard.app.
		SetRoot(dashboard.pages, true).
		SetFocus(dashboard.pages).
		Run(); err != nil {
		return err
	}

	return stopFunc()
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
		if err := d.logDataSource.Start(ctx); err != nil {
			return err
		}

		defer d.logDataSource.Stop() //nolint:errcheck

		lastLogTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case nodeLog := <-d.logDataSource.LogCh:
				if time.Since(lastLogTime) < 50*time.Millisecond {
					d.app.QueueUpdate(func() {
						d.processLog(nodeLog.Node, nodeLog.Log)
					})
				} else {
					d.app.QueueUpdateDraw(func() {
						d.processLog(nodeLog.Node, nodeLog.Log)
					})
				}

				lastLogTime = time.Now()
			case d.data = <-dataCh:
				d.app.QueueUpdateDraw(func() {
					d.processAPIData()
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

	for _, node := range maps.Keys(d.data.Nodes) {
		d.processSeenNode(node)
	}

	for _, component := range d.apiDataListeners {
		component.OnAPIDataChange(d.selectedNode, d.data)
	}
}

// processNodeResource re-renders the components with new resource data.
func (d *Dashboard) processNodeResource(nodeResource resourcedata.Data) {
	d.processSeenNode(nodeResource.Node)

	for _, component := range d.resourceDataListeners {
		component.OnResourceDataChange(nodeResource)
	}
}

// processLog re-renders the log components with new log data.
func (d *Dashboard) processLog(node, line string) {
	for _, component := range d.logDataListeners {
		component.OnLogDataChange(node, line)
	}
}

func (d *Dashboard) processSeenNode(node string) {
	_, exists := d.nodeSet[node]
	if exists {
		return
	}

	d.nodeSet[node] = struct{}{}

	nodes := maps.Keys(d.nodeSet)

	sort.Strings(nodes)

	d.nodes = nodes

	for _, listener := range d.nodeSetChangeListeners {
		listener.OnNodeSetChange(nodes)
	}

	// we received a new node, so we re-select the first node
	d.selectNodeByIndex(0)
}

func (d *Dashboard) selectScreen(screen Screen) {
	for _, info := range d.screenConfigs {
		info := info
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
