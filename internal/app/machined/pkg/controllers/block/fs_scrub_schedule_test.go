// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

const (
	volumeID   = "vol-var"
	mountpoint = "/var"
)

type FSScrubScheduleSuite struct {
	ctest.DefaultSuite
}

func TestFSScrubScheduleSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &FSScrubScheduleSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.FSScrubScheduleController{}))
			},
		},
	})
}

// createReadyXFSVolume creates a VolumeStatus + VolumeConfig pair for an XFS
// volume in the Ready phase, mounted at the given path. The pair is what the
// controller looks for: it walks VolumeStatuses, filters by phase/filesystem,
// and resolves the mountpoint via the matching VolumeConfig.
func (suite *FSScrubScheduleSuite) createReadyXFSVolume(volumeID, mountpoint string) {
	suite.T().Helper()

	volumeConfig := block.NewVolumeConfig(block.NamespaceName, volumeID)
	volumeConfig.TypedSpec().Mount.TargetPath = mountpoint
	suite.Create(volumeConfig)

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, volumeID)
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	volumeStatus.TypedSpec().Filesystem = block.FilesystemTypeXFS
	suite.Create(volumeStatus)
}

// TestNoVolumesNoSchedule ensures that with no volumes at all, no schedules are
// produced — the controller starts up cleanly and does nothing.
func (suite *FSScrubScheduleSuite) TestNoVolumesNoSchedule() {
	ctest.AssertNoResource[*block.FSScrubSchedule](suite, "any-id")
}

// TestNonReadyVolumeIgnored verifies that volumes that aren't in Ready phase
// are skipped (we only scrub mounted, ready filesystems).
func (suite *FSScrubScheduleSuite) TestNonReadyVolumeIgnored() {
	volumeConfig := block.NewVolumeConfig(block.NamespaceName, "vol-pending")
	volumeConfig.TypedSpec().Mount.TargetPath = mountpoint
	suite.Create(volumeConfig)

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, "vol-pending")
	volumeStatus.TypedSpec().Phase = block.VolumePhaseWaiting
	volumeStatus.TypedSpec().Filesystem = block.FilesystemTypeXFS
	suite.Create(volumeStatus)

	ctest.AssertNoResource[*block.FSScrubSchedule](suite, "vol-pending")
}

// TestNonXFSVolumeIgnored verifies that non-XFS filesystems are ignored —
// scrubbing is an XFS-specific operation.
func (suite *FSScrubScheduleSuite) TestNonXFSVolumeIgnored() {
	volumeConfig := block.NewVolumeConfig(block.NamespaceName, "vol-ext4")
	volumeConfig.TypedSpec().Mount.TargetPath = "/data"
	suite.Create(volumeConfig)

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, "vol-ext4")
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	volumeStatus.TypedSpec().Filesystem = block.FilesystemTypeEXT4
	suite.Create(volumeStatus)

	ctest.AssertNoResource[*block.FSScrubSchedule](suite, "vol-ext4")
}

// TestDefaultScheduleWithoutConfig is the headline default-behavior test:
// for a Ready XFS volume with NO matching FSScrubConfig, the controller should
// still produce a schedule using the default period.
func (suite *FSScrubScheduleSuite) TestDefaultScheduleWithoutConfig() {
	suite.createReadyXFSVolume(volumeID, mountpoint)

	ctest.AssertResource(suite, volumeID, func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal(mountpoint, s.TypedSpec().Mountpoint)
		asrt.Equal(blockcfg.DefaultScrubPeriod, s.TypedSpec().Period)
		// startTime must be in the future and within one period from now.
		asrt.True(s.TypedSpec().StartTime.After(time.Now()), "startTime should be in the future")
		asrt.True(s.TypedSpec().StartTime.Before(time.Now().Add(blockcfg.DefaultScrubPeriod+time.Minute)),
			"startTime should be within one period from now")
	})
}

// TestUserConfigOverridesDefault verifies that when the user supplies an
// FSScrubConfig for a mountpoint, that period wins over the default.
func (suite *FSScrubScheduleSuite) TestUserConfigOverridesDefault() {
	customPeriod := 1 * time.Hour

	cfg := block.NewFSScrubConfig("custom-cfg")
	cfg.TypedSpec().Mountpoint = mountpoint
	cfg.TypedSpec().Period = customPeriod
	suite.Create(cfg)

	suite.createReadyXFSVolume(volumeID, mountpoint)

	ctest.AssertResource(suite, volumeID, func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal(mountpoint, s.TypedSpec().Mountpoint)
		asrt.Equal(customPeriod, s.TypedSpec().Period)
	})
}

// TestConfigAddedFallsToCustom verifies that adding a config AFTER the default
// schedule has been created switches the schedule over to the user's period.
func (suite *FSScrubScheduleSuite) TestConfigAddedFallsToCustom() {
	suite.createReadyXFSVolume(volumeID, mountpoint)

	// Initial: default period.
	ctest.AssertResource(suite, volumeID, func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal(blockcfg.DefaultScrubPeriod, s.TypedSpec().Period)
	})

	// User adds a config — schedule should switch over.
	customPeriod := 30 * time.Minute
	cfg := block.NewFSScrubConfig("custom-cfg")
	cfg.TypedSpec().Mountpoint = mountpoint
	cfg.TypedSpec().Period = customPeriod
	suite.Create(cfg)

	ctest.AssertResource(suite, volumeID, func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal(customPeriod, s.TypedSpec().Period)
	})
}

// TestConfigRemovedFallsBackToDefault verifies the reverse: removing a custom
// config makes the schedule fall back to the default period.
func (suite *FSScrubScheduleSuite) TestConfigRemovedFallsBackToDefault() {
	customPeriod := 30 * time.Minute
	cfg := block.NewFSScrubConfig("custom-cfg")
	cfg.TypedSpec().Mountpoint = mountpoint
	cfg.TypedSpec().Period = customPeriod
	suite.Create(cfg)

	suite.createReadyXFSVolume(volumeID, mountpoint)

	ctest.AssertResource(suite, volumeID, func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal(customPeriod, s.TypedSpec().Period)
	})

	// Remove the user config.
	suite.Destroy(cfg)

	ctest.AssertResource(suite, volumeID, func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal(blockcfg.DefaultScrubPeriod, s.TypedSpec().Period)
	})
}

// TestVolumeRemovedDeschedules verifies that when a volume is no longer Ready
// (here: destroyed), its schedule is cleaned up.
func (suite *FSScrubScheduleSuite) TestVolumeRemovedDeschedules() {
	suite.createReadyXFSVolume(volumeID, mountpoint)

	ctest.AssertResource(suite, volumeID, func(*block.FSScrubSchedule, *assert.Assertions) {})

	// Tear down the volume.
	suite.Destroy(block.NewVolumeStatus(block.NamespaceName, volumeID))

	ctest.AssertNoResource[*block.FSScrubSchedule](suite, volumeID)
}

// TestMultipleVolumes verifies that multiple eligible volumes each get their
// own schedule, and that they have distinct (deterministic) start times — the
// hash-based phase computation should spread them across the period rather than
// firing them all at the same wall-clock instant.
func (suite *FSScrubScheduleSuite) TestMultipleVolumes() {
	suite.createReadyXFSVolume(volumeID, mountpoint)
	suite.createReadyXFSVolume("vol-data", "/var/lib/data")
	suite.createReadyXFSVolume("vol-logs", "/var/log")

	ctest.AssertResource(suite, volumeID, func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal(mountpoint, s.TypedSpec().Mountpoint)
		asrt.Equal(blockcfg.DefaultScrubPeriod, s.TypedSpec().Period)
	})
	ctest.AssertResource(suite, "vol-data", func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal("/var/lib/data", s.TypedSpec().Mountpoint)
	})
	ctest.AssertResource(suite, "vol-logs", func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal("/var/log", s.TypedSpec().Mountpoint)
	})
}

// TestDeterministicStartTimeAcrossReboots is the synctest-based determinism
// test. The promise is: for a given (path, period), the chosen startTime always
// lies on the same phase grid — i.e. (startTime.UnixNano() % period) is the
// same on every "reboot", regardless of when the reboot happens.
//
// We simulate two reboots at different virtual times and assert that the two
// startTimes differ only by an integer number of periods.
func TestDeterministicStartTimeAcrossReboots(t *testing.T) {
	t.Parallel()

	period := blockcfg.DefaultScrubPeriod

	// runOnce spins up the controller in a fresh state at the current bubble
	// time, lets it produce a schedule, captures the startTime, and tears
	// everything down — modeling a single boot.
	runOnce := func(t *testing.T) time.Time {
		t.Helper()

		suite := &ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.FSScrubScheduleController{}))
			},
		}

		suite.SetT(t) // bind to the synctest-bubble t

		suite.SetupTest()
		defer suite.TearDownTest()

		volumeConfig := block.NewVolumeConfig(block.NamespaceName, volumeID)
		volumeConfig.TypedSpec().Mount.TargetPath = mountpoint
		suite.Require().NoError(suite.State().Create(suite.Ctx(), volumeConfig))

		volumeStatus := block.NewVolumeStatus(block.NamespaceName, volumeID)
		volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
		volumeStatus.TypedSpec().Filesystem = block.FilesystemTypeXFS
		suite.Require().NoError(suite.State().Create(suite.Ctx(), volumeStatus))

		var startTime time.Time

		ctest.AssertResource(suite, volumeID, func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
			asrt.Equal(mountpoint, s.TypedSpec().Mountpoint)
			asrt.Equal(period, s.TypedSpec().Period)
			startTime = s.TypedSpec().StartTime
		})

		return startTime
	}

	synctest.Test(t, func(t *testing.T) {
		// Boot #1 at the bubble's start time (2000-01-01 UTC).
		first := runOnce(t)
		// Boot #1's startTime must be strictly in the future.
		assertInFuture(t, first, period)

		// Simulate a reboot at a non-trivial later time. We deliberately pick
		// a duration that is NOT a whole multiple of the period, so that if
		// the controller were re-randomizing each boot, the two startTimes
		// would not align modulo the period.
		time.Sleep(period + 37*time.Hour + 13*time.Minute)

		// Boot #2.
		second := runOnce(t)
		assertInFuture(t, second, period)

		// The crux of the determinism guarantee: the two start times must lie
		// on the same phase grid. Equivalently, their difference must be an
		// exact integer multiple of the period.
		diff := second.Sub(first)
		assert.Equal(t, time.Duration(0), diff%period,
			"startTimes %v and %v should differ by a whole number of periods (diff=%v, period=%v)",
			first, second, diff, period)

		// And the startTime must have actually advanced — it can't be in the
		// past, since each boot picks the *next* phase point.
		assert.Greater(t, diff, time.Duration(0),
			"second boot's startTime should be strictly later than first's")
	})
}

// TestDeterministicStartTimeDistinctPaths verifies that different paths produce
// different phases — one of the points of hashing the path is to spread out
// scrubs so they don't all fire at the same minute.
func TestDeterministicStartTimeDistinctPaths(t *testing.T) {
	t.Parallel()

	period := blockcfg.DefaultScrubPeriod

	runOnce := func(t *testing.T, volumeID, mountpoint string) time.Time {
		t.Helper()

		suite := &ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.FSScrubScheduleController{}))
			},
		}

		suite.SetT(t)

		suite.SetupTest()
		defer suite.TearDownTest()

		volumeConfig := block.NewVolumeConfig(block.NamespaceName, volumeID)
		volumeConfig.TypedSpec().Mount.TargetPath = mountpoint
		suite.Require().NoError(suite.State().Create(suite.Ctx(), volumeConfig))

		volumeStatus := block.NewVolumeStatus(block.NamespaceName, volumeID)
		volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
		volumeStatus.TypedSpec().Filesystem = block.FilesystemTypeXFS
		suite.Require().NoError(suite.State().Create(suite.Ctx(), volumeStatus))

		var startTime time.Time

		ctest.AssertResource(suite, volumeID, func(s *block.FSScrubSchedule, asrt *assert.Assertions) {
			startTime = s.TypedSpec().StartTime
		})

		return startTime
	}

	synctest.Test(t, func(t *testing.T) {
		varStart := runOnce(t, volumeID, mountpoint)
		dataStart := runOnce(t, "vol-data", "/var/lib/data")

		// Phase = startTime % period. Distinct paths should (with overwhelming
		// probability for any reasonable hash) map to distinct phases.
		varPhase := time.Duration(varStart.UnixNano()) % period
		dataPhase := time.Duration(dataStart.UnixNano()) % period

		assert.NotEqual(t, varPhase, dataPhase,
			"different mountpoints should hash to different phases within the period")
	})
}

// assertInFuture is a small helper that double-checks the invariant the
// controller promises: startTime > now and startTime ≤ now + period.
func assertInFuture(t *testing.T, startTime time.Time, period time.Duration) {
	t.Helper()

	now := time.Now()
	assert.True(t, startTime.After(now),
		"startTime %v should be strictly after now %v", startTime, now)
	assert.True(t, !startTime.After(now.Add(period)),
		"startTime %v should be at most one period (%v) past now %v", startTime, period, now)
}
