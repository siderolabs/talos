// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dashboard implements a text-based UI dashboard.
package dashboard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gizak/termui/v3"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/slices"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
	"github.com/siderolabs/talos/internal/pkg/dashboard/data"
	"github.com/siderolabs/talos/internal/pkg/dashboard/datasource"
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

// DataWidget is a widget which consumes Data to draw itself.
type DataWidget interface {
	Update(node string, data *data.Data)
}

// LogWidget is a widget which consumes logs and draws itself.
type LogWidget interface {
	UpdateLog(node string, logLine string)
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

	apiDataSource  *datasource.API
	logsDataSource *datasource.Logs

	dataWidgets         []DataWidget
	logWidgets          []LogWidget
	nodeSelectListeners []NodeSelectListener

	app *tview.Application

	mainGrid *tview.Grid

	summaryGrid       *SummaryGrid
	monitorGrid       *MonitorGrid
	networkConfigGrid *NetworkConfigGrid

	screenConfigs []screenConfig
	footer        *components.Footer

	initialDataReceived bool
	data                *data.Data
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
			dashboard.selectNode(-1)

			return nil
		case event.Key() == tcell.KeyRight, event.Rune() == 'l':
			dashboard.selectNode(+1)

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

	dashboard.dataWidgets = []DataWidget{
		header,
		dashboard.summaryGrid,
		dashboard.monitorGrid,
		dashboard.networkConfigGrid,
	}

	dashboard.logWidgets = []LogWidget{
		dashboard.summaryGrid,
	}

	dashboard.nodeSelectListeners = []NodeSelectListener{
		dashboard.summaryGrid,
	}

	dashboard.apiDataSource = &datasource.API{
		Client:   cli,
		Interval: defOptions.interval,
	}

	dashboard.logsDataSource = datasource.NewLogSource(cli)

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
		dataCh := d.apiDataSource.Run(ctx)
		defer d.apiDataSource.Stop()

		if err := d.logsDataSource.Start(ctx); err != nil {
			return err
		}

		defer d.logsDataSource.Stop() //nolint:errcheck

		lastLogTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case nodeLog := <-d.logsDataSource.LogCh:
				if time.Since(lastLogTime) < 50*time.Millisecond {
					d.app.QueueUpdate(func() {
						d.UpdateLogs(nodeLog.Node, nodeLog.Log)
					})
				} else {
					d.app.QueueUpdateDraw(func() {
						d.UpdateLogs(nodeLog.Node, nodeLog.Log)
					})
				}

				lastLogTime = time.Now()
			case d.data = <-dataCh:
				d.app.QueueUpdateDraw(func() {
					d.UpdateData()
				})
			}
		}
	})

	return stopFunc
}

func (d *Dashboard) selectNode(move int) {
	node := d.footer.SelectNode(move)

	d.UpdateData()

	for _, listener := range d.nodeSelectListeners {
		listener.OnNodeSelect(node)
	}
}

// UpdateData re-renders the widgets with new data.
func (d *Dashboard) UpdateData() {
	if d.data == nil {
		return
	}

	selectedNode := d.footer.UpdateNodes(maps.Keys(d.data.Nodes))

	for _, widget := range d.dataWidgets {
		widget.Update(selectedNode, d.data)
	}

	if !d.initialDataReceived {
		d.initialDataReceived = true
		d.selectNode(0)
	}
}

// UpdateLogs re-renders the log widgets with new log data.
func (d *Dashboard) UpdateLogs(node, line string) {
	for _, widget := range d.logWidgets {
		widget.UpdateLog(node, line)
	}
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
