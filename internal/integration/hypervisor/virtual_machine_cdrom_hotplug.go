// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"libvirt.org/go/libvirtxml"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const (
	mediaDiskName = "media"
	// mediaDevice is the second cdrom of the domain as the guest sees it.
	mediaDevice     = "/dev/sr1"
	mediaSectorSize = 2048
)

// domainID matches libvirt's domain ID, which changes only when the domain is replaced.
var domainID = regexp.MustCompile(`(?m)^Id:\s+(\d+)\s*$`)

// TestCDROMMediaChangeKeepsTheGuestRunning loads, swaps and ejects a medium without restarting the domain.
//
//nolint:gocyclo,cyclop
func (suite *CDROMSuite) TestCDROMMediaChangeKeepsTheGuestRunning() {
	node, nodeCtx := suite.requireAlpineGuestNode()

	doc := suite.prepareAlpineGuestOnNode(node, nodeCtx, nil, 512<<20, "")
	name := doc.Name()

	library := doc.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageLibrary
	pool := doc.DisksConfig[0].DiskPool

	firstDigest, firstNonce := suite.uploadMedium(nodeCtx, library, "media-a.iso")
	secondDigest, secondNonce := suite.uploadMedium(nodeCtx, library, "media-b.iso")

	// Watch, not poll: a short-lived definition would be missed between polls.
	recorder := suite.recordDomainSpec(nodeCtx, name)
	defer recorder.stop()

	// Adding the drive restarts the domain; only later changes are medium changes.
	suite.addEmptyMediaDrive(nodeCtx, doc)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 2, 0)
	suite.assertEmptyDriveReady(nodeCtx, doc)

	started := suite.domainID(node, name)

	console := suite.loginAlpineConsole(nodeCtx, name)
	defer func() {
		if suite.T().Failed() {
			suite.T().Logf("bounded Alpine console transcript: %q", console.transcript)
			recorder.report(suite.T())
		}
	}()

	suite.Assert().NoError(suite.guestTrayIsEmpty(console), "the drive starts out empty")

	suite.assertMediumWithoutPoolRejected(nodeCtx, doc, library, "media-a.iso", firstDigest)

	media := map[string]struct {
		digest string
		nonce  string
		source string
	}{
		"media-a.iso": {digest: firstDigest, nonce: firstNonce},
		"media-b.iso": {digest: secondDigest, nonce: secondNonce},
	}

	for _, medium := range []string{"media-a.iso", "media-b.iso"} {
		step := media[medium]
		step.source = suite.loadMedium(nodeCtx, doc, pool, library, medium, step.digest)
		media[medium] = step

		// Checked first: a replaced domain also drops the console.
		suite.Assert().Equal(started, suite.domainID(node, name),
			"libvirt replaced the domain instead of changing the medium to %q", medium)
		suite.Assert().NoError(suite.guestReadsMedium(console, step.nonce),
			"the guest did not read %q", medium)
	}

	// Linux locks the tray of an open cdrom; the change must wait, not be forced.
	suite.Assert().NoError(suite.guestLockTray(console), "the guest could not lock the tray")

	locked, loaded := media["media-b.iso"], media["media-a.iso"]
	loaded.source = suite.loadMedium(nodeCtx, doc, pool, library, "media-a.iso", loaded.digest)
	media["media-a.iso"] = loaded

	suite.Assert().False(suite.liveDefinitionReadsEventually(node, name, loaded.source, lockedTrayHold),
		"the medium was changed although the guest held the tray locked")
	suite.Assert().True(suite.liveDefinitionReadsEventually(node, name, locked.source, 0),
		"the running domain lost the medium the guest held locked")
	suite.Assert().Equal(started, suite.domainID(node, name), "libvirt replaced the domain whose tray was locked")
	suite.Assert().NoError(suite.guestReadsMedium(console, locked.nonce), "the guest lost the medium it held locked")

	suite.Assert().NoError(suite.guestUnlockTray(console), "the guest could not unlock the tray")
	suite.Assert().True(suite.liveDefinitionReadsEventually(node, name, loaded.source, mediaRetryWait),
		"the refused medium change was never retried once the tray was unlocked")
	suite.Assert().Equal(started, suite.domainID(node, name), "libvirt replaced the domain instead of retrying the medium change")
	suite.Assert().NoError(suite.guestReadsMedium(console, loaded.nonce), "the guest did not read the medium loaded after the unlock")

	suite.ejectMedium(nodeCtx, doc, "", loaded.source)
	suite.Assert().Equal(started, suite.domainID(node, name), "libvirt replaced the domain instead of ejecting the medium")
	suite.Assert().NoError(suite.guestTrayIsEmpty(console), "the guest still finds a medium after the eject")

	if suite.T().Failed() {
		return
	}

	suite.detachAlpineConsole(console)

	// Control: a memory change replaces the domain, so the ID checks above mean something.
	doc.MemoryConfig.MemorySize = meta.MustByteSize(strconv.FormatUint(640<<20, 10))
	suite.replaceVirtualMachineConfig(nodeCtx, doc)
	suite.Assert().Eventually(func() bool {
		id, ok := suite.tryDomainID(node, name)

		return ok && id != started
	}, 2*time.Minute, time.Second, "a memory change did not replace the domain")
}

// TestCDROMMediaChangeWhileAProfileEditIsPending checks a medium goes in live while a profile switch waits for restart.
//
//nolint:gocyclo
func (suite *CDROMSuite) TestCDROMMediaChangeWhileAProfileEditIsPending() {
	node, nodeCtx := suite.requireAlpineGuestNode()

	// The pending profile pins another versioned machine of the start profile's family: the live
	// machine shows whether it was applied, on every architecture. Builtin profiles cannot do that on
	// arm64, where "host" and "passthrough" render the same domain.
	arch := suite.guestArch(node)
	start := arch.profile("host")
	startedMachine, pendingMachine := suite.pendingMachineSwitch(node, arch.qemu, start.machine)

	profile := hypervisorcfg.NewVirtualMachineProfileConfigV1Alpha1()
	profile.MetaName = "pending-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	profile.MachineConfig = pendingMachine
	profile.CPUConfig.ModeConfig = start.mode

	// Registered before the guest so it runs after the guest's own cleanup: the guest restores a
	// config snapshot taken after this patch, which still contains the profile.
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cleanupCtx := client.WithNode(ctx, node)
		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, hypervisorcfg.VirtualMachineProfileConfigKind, profile.Name())
		rtestutils.AssertNoResource[*hypervisor.VirtualMachineProfileSpec](cleanupCtx, suite.T(), suite.Client.State(), profile.Name())
	})

	suite.PatchMachineConfig(nodeCtx, profile)

	doc := suite.prepareAlpineGuestOnNode(node, nodeCtx, nil, 512<<20, start.name)
	name := doc.Name()

	library := doc.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageLibrary
	pool := doc.DisksConfig[0].DiskPool

	mediumDigest, mediumNonce := suite.uploadMedium(nodeCtx, library, "media-a.iso")

	recorder := suite.recordDomainSpec(nodeCtx, name)
	defer recorder.stop()

	suite.addEmptyMediaDrive(nodeCtx, doc)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 2, 0)

	started := suite.domainID(node, name)
	suite.Require().Equal(startedMachine, suite.liveMachine(node, name), "libvirt resolved %q to another machine", start.machine)

	console := suite.loginAlpineConsole(nodeCtx, name)
	consoles := make([]*alpineConsole, 0, 2)
	consoles = append(consoles, console)

	defer func() {
		for _, attached := range consoles {
			if suite.T().Failed() {
				suite.T().Logf("bounded Alpine console transcript: %q", attached.transcript)
			}

			attached.cancel()
		}

		if suite.T().Failed() {
			recorder.report(suite.T())
		}
	}()

	doc.ProfileConfig = &profile.MetaName

	source := suite.loadMedium(nodeCtx, doc, pool, library, "media-a.iso", mediumDigest)

	suite.Assert().Equal(started, suite.domainID(node, name), "the medium change applied the pending profile by restarting")
	suite.Assert().Equal(startedMachine, suite.liveMachine(node, name), "the running guest changed its machine without a restart")
	suite.Assert().NoError(suite.guestReadsMedium(console, mediumNonce), "the guest did not read the medium")

	if suite.T().Failed() {
		return
	}

	suite.detachAlpineConsole(console)

	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	suite.replaceVirtualMachineConfig(nodeCtx, doc)
	suite.assertStoppedDomain(nodeCtx, node, name)

	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	suite.replaceVirtualMachineConfig(nodeCtx, doc)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 2, 0)

	suite.Assert().Equal(pendingMachine, suite.liveMachine(node, name), "the pending profile was not applied at the restart")
	suite.Assert().True(suite.liveDefinitionReadsEventually(node, name, source, 0), "the medium did not survive the restart")

	console = suite.loginAlpineConsole(nodeCtx, name)
	consoles = append(consoles, console)

	suite.Assert().NoError(suite.guestReadsMedium(console, mediumNonce), "the guest did not read the medium after the restart")

	if suite.T().Failed() {
		return
	}

	suite.detachAlpineConsole(console)
}

const (
	// lockedTrayHold outlasts one change attempt plus a retry.
	lockedTrayHold = 45 * time.Second
	mediaRetryWait = 2 * time.Minute
)

// addEmptyMediaDrive appends an empty media drive with no pool.
func (suite *LibvirtSuite) addEmptyMediaDrive(nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) {
	suite.T().Helper()

	doc.DisksConfig = append(doc.DisksConfig, hypervisorcfg.VirtualMachineDisk{
		DiskName: mediaDiskName,
		DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
		ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
			BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{},
		},
	})

	suite.replaceVirtualMachineConfig(nodeCtx, doc)
}

// assertEmptyDriveReady waits for the empty media drive to be ready.
func (suite *LibvirtSuite) assertEmptyDriveReady(nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) {
	suite.T().Helper()

	disk := doc.DisksConfig[len(doc.DisksConfig)-1]
	suite.Require().Equal(mediaDiskName, disk.DiskName)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), diskStatusID(doc.Name(), disk),
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, status.TypedSpec().Phase, "error: %q", status.TypedSpec().Error)
			asrt.Empty(status.TypedSpec().Pool)
			asrt.Empty(status.TypedSpec().SourcePath)
		},
	)
}

// assertMediumWithoutPoolRejected expects a medium with no pool to be refused.
func (suite *LibvirtSuite) assertMediumWithoutPoolRejected(
	nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1, library, file, pinned string,
) {
	suite.T().Helper()

	rejected := doc.DeepCopy()

	for i := range rejected.DisksConfig {
		if rejected.DisksConfig[i].DiskName == mediaDiskName {
			rejected.DisksConfig[i].DiskPool = ""
			rejected.DisksConfig[i].ProvisionConfig = hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: library,
					ImageFile:    file,
					ImageDigest:  pinned,
				},
			}
		}
	}

	err := suite.applyVirtualMachineConfig(nodeCtx, rejected)
	suite.Assert().ErrorContains(err, "pool is required", "a medium was accepted into a drive naming no pool")
}

// tryVirsh runs virsh, reporting failure instead of failing the test.
func (suite *LibvirtSuite) tryVirsh(node string, args ...string) (string, bool) {
	output, code := suite.RunDebugContainer(suite.ctx, node, append([]string{"/usr/local/bin/virsh", "--connect", libvirtURI}, args...)...)

	return output, code == 0
}

// tryDomainID reads the running domain's ID, if any.
func (suite *LibvirtSuite) tryDomainID(node, name string) (string, bool) {
	text, ok := suite.tryVirsh(node, "dominfo", name)
	if !ok {
		return "", false
	}

	match := domainID.FindStringSubmatch(text)
	if len(match) != 2 {
		return "", false
	}

	return match[1], true
}

// liveDefinitionReadsEventually reports whether the running domain reads source within the timeout.
func (suite *LibvirtSuite) liveDefinitionReadsEventually(node, name, source string, within time.Duration) bool {
	deadline := time.Now().Add(within)

	for {
		if text, ok := suite.tryVirsh(node, "dumpxml", name); ok && strings.Contains(text, source) {
			return true
		}

		if !time.Now().Before(deadline) {
			return false
		}

		time.Sleep(2 * time.Second)
	}
}

// liveMachine reads the running domain's machine type; libvirt resolves aliases to versioned machines.
func (suite *LibvirtSuite) liveMachine(node, name string) string {
	suite.T().Helper()

	var domain libvirtxml.Domain
	suite.Require().NoError(domain.Unmarshal(suite.runVirsh(node, "dumpxml", name)))

	if domain.OS == nil || domain.OS.Type == nil {
		return ""
	}

	return domain.OS.Type.Machine
}

// pendingMachineSwitch returns the versioned machine libvirt resolves the alias to, and another
// supported, non-deprecated versioned machine of the same family to switch to.
func (suite *LibvirtSuite) pendingMachineSwitch(node, guestArch, alias string) (current, other string) {
	suite.T().Helper()

	var caps libvirtxml.Caps
	suite.Require().NoError(caps.Unmarshal(suite.runVirsh(node, "capabilities")))

	var machines []libvirtxml.CapsGuestMachine

	for _, guest := range caps.Guests {
		if guest.Arch.Name != guestArch {
			continue
		}

		machines = append(machines, guest.Arch.Machines...)

		for _, domain := range guest.Arch.Domains {
			machines = append(machines, domain.Machines...)
		}
	}

	for _, machine := range machines {
		if machine.Name == alias && machine.Canonical != "" {
			current = machine.Canonical

			break
		}
	}

	suite.Require().NotEmpty(current, "libvirt reports no versioned machine for %q on %s", alias, guestArch)

	// pc-q35-10.1 -> pc-q35-, virt-10.1 -> virt-.
	family := current[:strings.LastIndex(current, "-")+1]

	// Any other release will do: only the live machine name has to differ.
	for _, machine := range machines {
		if machine.Name != current && machine.Deprecated != "yes" && machine.Canonical == "" &&
			strings.HasPrefix(machine.Name, family) {
			return current, machine.Name
		}
	}

	suite.Require().FailNow("no machine to switch to", "libvirt reports no other %s* machine on %s", family, guestArch)

	return "", ""
}

// guestLockTray locks the tray by holding the drive open in the guest.
func (suite *LibvirtSuite) guestLockTray(console *alpineConsole) error {
	suite.T().Helper()

	return suite.runGuestProbe(console, fmt.Sprintf("exec 3<%s", mediaDevice),
		"TRAY_LOCKED_"+newNonce())
}

// guestUnlockTray undoes guestLockTray.
func (suite *LibvirtSuite) guestUnlockTray(console *alpineConsole) error {
	suite.T().Helper()

	return suite.runGuestProbe(console, "exec 3<&-",
		"TRAY_UNLOCKED_"+newNonce())
}

// uploadMedium uploads a one-sector medium, returning its digest and nonce.
func (suite *LibvirtSuite) uploadMedium(nodeCtx context.Context, library, file string) (string, string) {
	suite.T().Helper()

	nonce := newNonce()

	contents := make([]byte, mediaSectorSize)
	copy(contents, "MEDIUM_"+nonce)

	pinned := digest.FromBytes(contents).String()

	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, file, pinned, bytes.NewReader(contents))
	suite.Require().NoError(err)

	return pinned, nonce
}

// loadMedium loads a library file into the drive and returns the pool copy's path once the definition uses it.
func (suite *LibvirtSuite) loadMedium(
	nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1, pool, library, file, pinned string,
) string {
	suite.T().Helper()

	disk := suite.setMediaProvision(nodeCtx, doc, pool, hypervisorcfg.VirtualMachineDiskProvision{
		FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
			ImageLibrary: library,
			ImageFile:    file,
			ImageDigest:  pinned,
		},
	})

	var source string

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), diskStatusID(doc.Name(), disk),
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, status.TypedSpec().Phase, "error: %q", status.TypedSpec().Error)

			source = status.TypedSpec().SourcePath
			asrt.NotEmpty(source)
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), doc.Name(),
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Contains(spec.TypedSpec().DomainXML, source, "the definition does not read %q", file)
		},
	)

	return source
}

// ejectMedium empties the media drive.
func (suite *LibvirtSuite) ejectMedium(nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1, pool, source string) {
	suite.T().Helper()

	suite.setMediaProvision(nodeCtx, doc, pool, hypervisorcfg.VirtualMachineDiskProvision{
		BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{},
	})

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), doc.Name(),
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.NotContains(spec.TypedSpec().DomainXML, source)
		},
	)
}

// setMediaProvision sets the media drive's provision and pool, returning the drive as applied.
func (suite *LibvirtSuite) setMediaProvision(
	nodeCtx context.Context,
	doc *hypervisorcfg.VirtualMachineConfigV1Alpha1, pool string, provision hypervisorcfg.VirtualMachineDiskProvision,
) hypervisorcfg.VirtualMachineDisk {
	suite.T().Helper()

	for i := range doc.DisksConfig {
		if doc.DisksConfig[i].DiskName == mediaDiskName {
			doc.DisksConfig[i].DiskPool = pool
			doc.DisksConfig[i].ProvisionConfig = provision

			suite.replaceVirtualMachineConfig(nodeCtx, doc)

			return doc.DisksConfig[i]
		}
	}

	suite.Require().FailNow("the virtual machine has no media drive to change")

	return hypervisorcfg.VirtualMachineDisk{}
}

// replaceVirtualMachineConfig replaces the same-named document; a patch would merge disk sources.
func (suite *LibvirtSuite) replaceVirtualMachineConfig(nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) {
	suite.T().Helper()

	suite.Require().NoError(suite.applyVirtualMachineConfig(nodeCtx, doc))
}

// applyVirtualMachineConfig is replaceVirtualMachineConfig returning the apply error; any other error fails the test.
func (suite *LibvirtSuite) applyVirtualMachineConfig(nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) error {
	suite.T().Helper()

	current, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	documents := make([]config.Document, 0, len(current.Documents())+1)

	for _, document := range current.Documents() {
		named, ok := document.(config.NamedDocument)
		if ok && document.Kind() == hypervisorcfg.VirtualMachineConfigKind && named.Name() == doc.Name() {
			continue
		}

		documents = append(documents, document)
	}

	replaced, err := container.New(append(documents, doc)...)
	suite.Require().NoError(err)

	data, err := replaced.Bytes()
	suite.Require().NoError(err)

	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		Data: data,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})

	return err
}

// domainID reads the running domain's ID.
func (suite *LibvirtSuite) domainID(node, name string) string {
	suite.T().Helper()

	id, ok := suite.tryDomainID(node, name)
	suite.Require().True(ok, "virsh dominfo %q reported no ID", name)

	return id
}

// guestReadsMedium waits for the guest to read the medium; the first read after a change may fail.
func (suite *LibvirtSuite) guestReadsMedium(console *alpineConsole, nonce string) error {
	suite.T().Helper()

	return suite.runGuestProbe(console,
		fmt.Sprintf("dd if=%s bs=%d count=1 2>/dev/null | tr -d '\\0' | grep -q MEDIUM_%s", mediaDevice, mediaSectorSize, nonce),
		"MEDIUM_READ_"+nonce)
}

// guestTrayIsEmpty waits for the guest to find the drive empty.
func (suite *LibvirtSuite) guestTrayIsEmpty(console *alpineConsole) error {
	suite.T().Helper()

	return suite.runGuestProbe(console,
		fmt.Sprintf("! dd if=%s bs=%d count=1 >/dev/null 2>&1", mediaDevice, mediaSectorSize),
		"MEDIUM_ABSENT_"+newNonce())
}

// runGuestProbe retries a shell test in the guest until it holds; the marker is split to dodge the echo.
func (suite *LibvirtSuite) runGuestProbe(console *alpineConsole, probe, marker string) error {
	suite.T().Helper()

	prefix, suffix, found := strings.Cut(marker, "_")
	suite.Require().True(found)

	start := console.mark()

	if err := console.send(fmt.Sprintf(
		"for i in $(seq 1 30); do if %s; then printf '\\n%s_%%s\\n' '%s'; break; fi; sleep 1; done\n",
		probe, prefix, suffix)); err != nil {
		return fmt.Errorf("send guest probe %q: %w", marker, err)
	}

	return console.expectAfter(start, []byte("\r\n"+marker+"\r\n"))
}

// domainSpecRecorder records every domain spec and status transition, for failure reports.
type domainSpecRecorder struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu          sync.Mutex
	transitions []string
}

// recordDomainSpec starts a domainSpecRecorder for name.
func (suite *LibvirtSuite) recordDomainSpec(nodeCtx context.Context, name string) *domainSpecRecorder {
	suite.T().Helper()

	ctx, cancel := context.WithCancel(nodeCtx)
	recorder := &domainSpecRecorder{cancel: cancel, done: make(chan struct{})}

	events := make(chan state.Event)

	suite.Require().NoError(suite.Client.State().Watch(ctx,
		hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name).Metadata(), events))
	suite.Require().NoError(suite.Client.State().Watch(ctx,
		hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, name).Metadata(), events))

	go recorder.collect(ctx, events)

	return recorder
}

func (recorder *domainSpecRecorder) collect(ctx context.Context, events <-chan state.Event) {
	defer close(recorder.done)

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			recorder.record(event)
		}
	}
}

// record logs one event as a single line.
func (recorder *domainSpecRecorder) record(event state.Event) {
	var line string

	switch res := event.Resource.(type) {
	case *hypervisor.VirtualMachineDomainSpec:
		line = fmt.Sprintf("%-9s spec      version=%s powerState=%q definition=%s disks=%v cloudInit=%q",
			event.Type, res.Metadata().Version(), res.TypedSpec().PowerState,
			shortDigest(res.TypedSpec().DomainXML), res.TypedSpec().Disks, res.TypedSpec().CloudInit)
	case *hypervisor.VirtualMachineDomainStatus:
		line = fmt.Sprintf("%-9s domain    version=%s powerState=%q state=%d error=%q",
			event.Type, res.Metadata().Version(), res.TypedSpec().PowerState,
			res.TypedSpec().State, res.TypedSpec().Error)
	default:
		if event.Error != nil {
			line = fmt.Sprintf("%-9s watch error: %v", event.Type, event.Error)
		} else {
			line = fmt.Sprintf("%-9s %T", event.Type, event.Resource)
		}
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()

	recorder.transitions = append(recorder.transitions, line)
}

// shortDigest abbreviates the hash of a definition.
func shortDigest(text string) string {
	if text == "" {
		return "(none)"
	}

	sum := sha256.Sum256([]byte(text))

	return hex.EncodeToString(sum[:])[:12]
}

// report logs every recorded transition, oldest first.
func (recorder *domainSpecRecorder) report(t *testing.T) {
	t.Helper()

	recorder.mu.Lock()
	defer recorder.mu.Unlock()

	t.Logf("virtual machine definition transitions (%d):\n%s",
		len(recorder.transitions), strings.Join(recorder.transitions, "\n"))
}

func (recorder *domainSpecRecorder) stop() {
	recorder.cancel()
	<-recorder.done
}

// newNonce returns a fresh probe marker.
func newNonce() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}
