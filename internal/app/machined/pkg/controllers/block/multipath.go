// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/pmorjan/kmod"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/devicemapper"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/dmmultipath"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/sysblock"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// MultipathdServiceID is the ID of the service of the multipath-tools system extension.
const MultipathdServiceID = "ext-multipathd"

// sysBlockPath is where the kernel lists the block devices.
const sysBlockPath = "/sys/block"

// multipathResyncInterval is how often the paths are checked for a change of their state, which
// the kernel doesn't announce with an event.
const multipathResyncInterval = 10 * time.Second

// multipathRetryInterval is how soon a multipath device which failed to be assembled is retried
// first; the interval doubles with every failure in a row, up to multipathMaxRetryInterval.
const (
	multipathRetryInterval    = 5 * time.Second
	multipathMaxRetryInterval = time.Minute
)

// multipathModules are the kernel modules a multipath device is made of: the target, and the path
// selector which spreads the I/O over the paths.
//
// The kernel requests the module of a target when a table refers to it, but the modules are loaded
// upfront, so that a missing module is reported as such rather than as a table the kernel rejects.
var multipathModules = []string{"dm-multipath", "dm-round-robin"}

// NativeMultipathEnabled reports whether the kernel command line asks Talos to assemble multipath
// devices itself (see constants.KernelParamMultipath).
func NativeMultipathEnabled(cmdline *procfs.Cmdline) bool {
	if cmdline == nil {
		return false
	}

	value := cmdline.Get(constants.KernelParamMultipath).First()
	if value == nil {
		return false
	}

	switch strings.ToLower(*value) {
	case "on":
		return true
	case "off":
		return false
	}

	enabled, _ := strconv.ParseBool(*value) //nolint:errcheck

	return enabled
}

// MultipathController assembles multipath devices over the disks which are paths to the same
// logical unit.
//
// A multipath device has to be in place before anything on its paths is put to use, as a disk
// which is in use can't become a path of a multipath device anymore. The system disk is put to use
// long before any system extension (e.g. multipath-tools) is started, as it holds the machine
// configuration the extensions are configured with, so it is Talos which assembles the multipath
// devices early in the boot.
//
// The multipath devices are assembled exactly as multipath-tools would: they are named after the
// WWID of the disk, and carry the same UUID, so that multipathd adopts them once it is started. From
// then on, they are left to multipathd entirely.
//
// Until then, Talos keeps the paths of each multipath device up to date: a new path is added as
// soon as it appears, and a path which is gone or can't be reached is removed. As the kernel never
// reinstates a path it failed on its own, a failed path which serves I/O once again is reinstated.
type MultipathController struct {
	// Enabled is set when Talos assembles multipath devices (see NativeMultipathEnabled).
	Enabled bool
}

// Name implements controller.Controller interface.
func (ctrl *MultipathController) Name() string {
	return "block.MultipathController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MultipathController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.DeviceType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(MultipathdServiceID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MultipathController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *MultipathController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if !ctrl.Enabled {
		return nil
	}

	loadMultipathModules(logger)

	var control *devicemapper.Control

	defer func() {
		if control != nil {
			control.Close() //nolint:errcheck
		}
	}()

	resyncTicker := time.NewTicker(multipathResyncInterval)
	defer resyncTicker.Stop()

	var (
		retryCh       <-chan time.Time
		retryInterval = multipathRetryInterval
		handedOff     bool
	)

	// the problems reported for each multipath device, so that a problem which persists is only
	// reported once rather than on every resync
	problems := map[string]string{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-resyncTicker.C:
		case <-retryCh:
		}

		retryCh = nil

		multipathdRunning, err := ctrl.multipathdRunning(ctx, r)
		if err != nil {
			return err
		}

		if multipathdRunning != handedOff {
			handedOff = multipathdRunning

			if handedOff {
				logger.Info("multipathd is running, the multipath devices are left to it")
			} else {
				logger.Info("multipathd is not running, managing the multipath devices")
			}
		}

		if handedOff {
			continue
		}

		if control == nil {
			if control, err = devicemapper.NewControl(); err != nil {
				return fmt.Errorf("failed to open the device-mapper control device: %w", err)
			}
		}

		devices, err := safe.ReaderListAll[*block.Device](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list block devices: %w", err)
		}

		if ctrl.reconcile(ctx, logger, control, devices, problems) {
			retryInterval = multipathRetryInterval
		} else {
			retryCh = time.After(retryInterval)
			retryInterval = min(2*retryInterval, multipathMaxRetryInterval)
		}
	}
}

// loadMultipathModules loads the kernel modules of a multipath device.
//
// A failure is not fatal: the modules might be built into the kernel, and if they are missing,
// assembling a multipath device fails with an error of its own.
func loadMultipathModules(logger *zap.Logger) {
	manager, err := kmod.New()
	if err != nil {
		logger.Warn("failed to initialize the kernel module manager", zap.Error(err))

		return
	}

	for _, module := range multipathModules {
		if err = manager.Load(module, "", 0); err != nil {
			logger.Warn("failed to load kernel module", zap.String("module", module), zap.Error(err))
		}
	}
}

func (ctrl *MultipathController) multipathdRunning(ctx context.Context, r controller.Reader) (bool, error) {
	service, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, MultipathdServiceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get the multipathd service: %w", err)
	}

	return service.TypedSpec().Running, nil
}

// reconcile assembles a multipath device for each disk which has paths, or updates its paths.
//
// It returns false if any multipath device failed to be assembled.
//
//nolint:gocyclo
func (ctrl *MultipathController) reconcile(
	ctx context.Context, logger *zap.Logger, control *devicemapper.Control, devices safe.List[*block.Device], problems map[string]string,
) bool {
	byID := map[string]*block.Device{}

	for device := range devices.All() {
		byID[device.Metadata().ID()] = device
	}

	ok := true

	groups := sysblock.MultipathGroups(devices.All())

	// forget the problems of the disks which are gone
	maps.DeleteFunc(problems, func(wwid, _ string) bool {
		_, present := groups[wwid]

		return !present
	})

	for wwid, ids := range groups {
		logger := logger.With(zap.String("wwid", wwid))

		paths := xslices.Map(ids, func(id string) *block.Device { return byID[id] })

		table, activePaths, err := multipathTable(paths)
		if err != nil {
			if reportProblem(problems, wwid, err) {
				logger.Warn("not assembling the multipath device", zap.Strings("paths", ids), zap.Error(err))
			}

			continue
		}

		if len(activePaths) == 0 {
			// no path can be reached right now: keep whatever the multipath device is made of, there
			// is nothing better to replace it with
			continue
		}

		uuid := sysblock.MultipathUUID(wwid)

		// the multipath device is looked up in the kernel rather than among the block devices: the
		// block device of a multipath device which has just been created, or the paths of one which
		// has just been reloaded, are only updated once the kernel event is processed
		existing, existingPaths, isAssembled := sysblock.FindDeviceMapper(sysBlockPath, uuid)
		if isAssembled && slices.Equal(existingPaths, activePaths) {
			delete(problems, wwid)

			ctrl.reinstatePaths(ctx, logger, uuid, paths)

			continue
		}

		if isAssembled {
			err = control.ReloadDevice(devicemapper.ByUUID(uuid), table)
		} else {
			err = control.CreateDevice(devicemapper.DeviceID{Name: wwid, UUID: uuid}, table)
		}

		if err != nil {
			if reportProblem(problems, wwid, err) {
				logger.Error("failed to assemble the multipath device", zap.Strings("paths", activePaths), zap.Error(err))
			}

			ok = false

			continue
		}

		delete(problems, wwid)

		if isAssembled {
			logger.Info("updated the paths of the multipath device", zap.String("device", existing), zap.Strings("paths", activePaths))
		} else {
			logger.Info("assembled the multipath device", zap.Strings("paths", activePaths))
		}
	}

	return ok
}

// reportProblem records the problem of the multipath device, and reports whether it is a new one.
func reportProblem(problems map[string]string, wwid string, err error) bool {
	if problems[wwid] == err.Error() {
		return false
	}

	problems[wwid] = err.Error()

	return true
}

// pathCheckTimeout is how long a failed path is given to serve a read before it is considered unreachable.
const pathCheckTimeout = 5 * time.Second

// reinstatePaths brings back the paths of the multipath device which the kernel has failed, but
// which serve I/O once again.
//
// The kernel fails a path on an I/O error, and never brings it back on its own. A path which
// can't be reached for a while (its SCSI device leaves the running state) is dropped from the
// multipath device altogether, and added back once it is reachable again, but a path might also
// fail while its SCSI device stays running (e.g. on a transient error of the fabric).
func (ctrl *MultipathController) reinstatePaths(ctx context.Context, logger *zap.Logger, uuid string, paths []*block.Device) {
	states, err := dmmultipath.PathStates(uuid)
	if err != nil {
		logger.Warn("failed to get the state of the paths", zap.Error(err))

		return
	}

	for _, path := range paths {
		devNo := fmt.Sprintf("%d:%d", path.TypedSpec().Major, path.TypedSpec().Minor)

		if active, ok := states[devNo]; !ok || active {
			continue
		}

		if err = dmmultipath.CheckPath(ctx, filepath.Join("/dev", path.Metadata().ID()), pathCheckTimeout); err != nil {
			logger.Debug("failed path is still unreachable", zap.String("path", path.Metadata().ID()), zap.Error(err))

			continue
		}

		if err = dmmultipath.ReinstatePath(uuid, devNo); err != nil {
			logger.Warn("failed to reinstate the path", zap.String("path", path.Metadata().ID()), zap.Error(err))

			continue
		}

		logger.Info("reinstated the path", zap.String("path", path.Metadata().ID()))
	}
}

// multipathTable returns the device-mapper table of a multipath device over the paths which can be
// reached, along with their IDs.
//
// All paths are in a single path group, and I/O is spread over them round-robin. A path which is
// not reachable right now is left out, as the kernel would only fail it on the first I/O anyway.
// The hardware handler the kernel has attached to the paths (e.g. ALUA), if any, is kept.
//
// Once no path is left, I/O fails rather than queues: there is nothing to reinstate a path which
// failed on an I/O error, so it would queue forever.
func multipathTable(paths []*block.Device) ([]devicemapper.Target, []string, error) {
	var (
		sectors uint64
		active  []string
		params  []string
	)

	for _, path := range paths {
		pathSectors := sysblock.Sectors(path.TypedSpec().DevicePath)
		if pathSectors == 0 {
			// a path which reports no size is not ready (e.g. the logical unit is not spun up)
			continue
		}

		if sectors != 0 && pathSectors != sectors {
			return nil, nil, fmt.Errorf("paths report different sizes: %d and %d sectors", sectors, pathSectors)
		}

		sectors = pathSectors

		if !sysblock.SCSIDeviceRunning(path.TypedSpec().DevicePath) {
			continue
		}

		active = append(active, path.Metadata().ID())
		params = append(params, fmt.Sprintf("%d:%d 1", path.TypedSpec().Major, path.TypedSpec().Minor))
	}

	if len(active) == 0 {
		return nil, nil, nil
	}

	return []devicemapper.Target{
		{
			Type: "multipath",
			// <#features> <#hw handler args> <#path groups> <initial path group>
			// <path selector> <#selector args> <#paths> <#path args> [<path> <repeat count>]...
			Params: fmt.Sprintf("0 0 1 1 round-robin 0 %d 1 %s", len(active), strings.Join(params, " ")),
			Length: sectors,
		},
	}, active, nil
}
