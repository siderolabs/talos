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
	// ScreenSummary is the summary screen.
	ScreenSummary Screen = "Summary"

	// ScreenMonitor is the monitor (metrics) screen.
	ScreenMonitor Screen = "Monitor"

	// ScreenNetworkConfig is the network configuration screen.
	ScreenNetworkConfig Screen = "Network Config"
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
	screenKey string
	screen    Screen
	keyCode   tcell.Key
	primitive screenSelectListener
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

	summaryGrid       *SummaryGrid
	monitorGrid       *MonitorGrid
	networkConfigGrid *NetworkConfigGrid

	screenConfigs []screenConfig
	footer        *components.Footer

	data *apidata.Data

	selectedNodeIndex int
	selectedNode      string
	nodeSet           map[string]struct{}
	nodes             []string
}

// New initializes the summary dashboard.
func New(cli *client.Client, opts ...Option) (*Dashboard, error) {
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

	header := components.NewHeader()
	dashboard.mainGrid.AddItem(header, 0, 0, 1, 1, 0, 0, false)

	dashboard.summaryGrid = NewSummaryGrid(dashboard.app)
	dashboard.monitorGrid = NewMonitorGrid(dashboard.app)
	dashboard.networkConfigGrid = NewNetworkConfigGrid(dashboard.app)

	err := dashboard.initScreenConfigs(defOptions.screens)
	if err != nil {
		return nil, err
	}

	screenKeyToName := slices.ToMap(dashboard.screenConfigs, func(t screenConfig) (string, string) {
		return t.screenKey, string(t.screen)
	})

	screenKeyCodeToScreen := slices.ToMap(dashboard.screenConfigs, func(t screenConfig) (tcell.Key, Screen) {
		return t.keyCode, t.screen
	})

	dashboard.footer = components.NewFooter(screenKeyToName)

	dashboard.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		screenName, screenOk := screenKeyCodeToScreen[event.Key()]

		switch {
		case screenOk:
			dashboard.selectScreen(screenName)
		case event.Key() == tcell.KeyLeft, event.Rune() == 'h':
			dashboard.selectNodeByIndex(dashboard.selectedNodeIndex - 1)

			return nil
		case event.Key() == tcell.KeyRight, event.Rune() == 'l':
			dashboard.selectNodeByIndex(dashboard.selectedNodeIndex + 1)

			return nil
		case event.Key() == tcell.KeyCtrlC, event.Rune() == 'q':
			if defOptions.allowExitKeys {
				dashboard.app.Stop()
			}

			return nil
		}

		return event
	})

	dashboard.mainGrid.AddItem(dashboard.footer, 2, 0, 1, 1, 0, 0, false)

	dashboard.apiDataListeners = []APIDataListener{
		header,
		dashboard.summaryGrid,
		dashboard.monitorGrid,
	}

	dashboard.resourceDataListeners = []ResourceDataListener{
		dashboard.summaryGrid,
		dashboard.networkConfigGrid,
	}

	dashboard.logDataListeners = []LogDataListener{
		dashboard.summaryGrid,
	}

	dashboard.nodeSelectListeners = []NodeSelectListener{
		dashboard.summaryGrid,
		dashboard.footer,
	}

	dashboard.nodeSetChangeListeners = []NodeSetListener{
		dashboard.footer,
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

func (d *Dashboard) initScreenConfigs(screens []Screen) error {
	primitiveForScreen := func(screen Screen) screenSelectListener {
		switch screen {
		case ScreenSummary:
			return d.summaryGrid
		case ScreenMonitor:
			return d.monitorGrid
		case ScreenNetworkConfig:
			return d.networkConfigGrid
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

		d.screenConfigs = append(d.screenConfigs, screenConfig{
			screenKey: fmt.Sprintf("F%d", i+1),
			screen:    screen,
			keyCode:   tcell.KeyF1 + tcell.Key(i),
			primitive: primitive,
		})
	}

	return nil
}

// Run starts the dashboard.
func (d *Dashboard) Run(ctx context.Context) error {
	d.selectScreen(ScreenSummary)

	stopFunc := d.startDataHandler(ctx)
	defer stopFunc() //nolint:errcheck

	if err := d.app.SetRoot(d.mainGrid, true).SetFocus(d.mainGrid).Run(); err != nil {
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
		if info.screen == screen {
			d.mainGrid.AddItem(info.primitive, 1, 0, 1, 1, 0, 0, false)

			info.primitive.onScreenSelect(true)

			continue
		}

		d.mainGrid.RemoveItem(info.primitive)
		info.primitive.onScreenSelect(false)
	}

	d.footer.SelectScreen(string(screen))
}
