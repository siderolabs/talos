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
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const (
	// mediaDiskName is the drive the medium is loaded into and ejected from. The guest boots from
	// another drive, so nothing here depends on what is in this one.
	mediaDiskName = "media"
	// mediaDevice is the guest device that drive appears as: the second cdrom of the domain, and
	// the guest numbers its optical drives in the order the domain presents them.
	mediaDevice = "/dev/sr1"
	// mediaSectorSize is what a read of one cdrom sector returns, so a medium is padded to it.
	mediaSectorSize = 2048
)

// domainID is libvirt's ID for a running domain. It is assigned when the domain starts, so a domain
// which was replaced rather than changed in place reports a new one.
var domainID = regexp.MustCompile(`(?m)^Id:\s+(\d+)\s*$`)

// TestCDROMMediaChangeKeepsTheGuestRunning loads, swaps and ejects a medium in a drive of a booted
// guest, and proves that none of it restarted the domain.
//
// The drive starts out empty and names no pool, which an empty drive needs none of; loading a
// medium into it without naming one is refused before anything reaches the node's guests. A change
// the guest holds off by keeping its tray locked waits, with the guest left running, until the
// guest lets go. A change which cannot be made in place restarts the domain, which is what makes
// an unchanged domain ID mean anything in the steps before it.
//
// The proof is in two halves which have to agree: libvirt still reports the ID it assigned when the
// domain started, and the guest -- which would have lost its uptime to a restart -- reads each new
// medium as it arrives.
//
// The two are checked in that order and neither aborts the other. A guest which lost its console
// says nothing about why on its own, so the domain identity is read first and every observation is
// collected before the case is allowed to fail: a replaced domain and a dropped console look alike
// from the console end, and only the recorded definition says which happened.
//
//nolint:gocyclo,cyclop
func (suite *LibvirtSuite) TestCDROMMediaChangeKeepsTheGuestRunning() {
	node, nodeCtx := suite.requireAlpineGuestNode()

	// No DHCP reservation: this case needs a booted guest with a console, not a network.
	doc := suite.prepareAlpineGuestOnNode(node, nodeCtx, nil, 512<<20, "")
	name := doc.Name()

	library := doc.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageLibrary
	pool := doc.DisksConfig[0].DiskPool

	firstDigest, firstNonce := suite.uploadMedium(nodeCtx, library, "media-a.iso")
	secondDigest, secondNonce := suite.uploadMedium(nodeCtx, library, "media-b.iso")

	// Started before the first edit, so every definition the node went through while the medium
	// moved is on the record, including one it held for less time than anything here could poll.
	recorder := suite.recordDomainSpec(nodeCtx, name)
	defer recorder.stop()

	// The drive itself is a device, so adding it redefines the domain and the guest reboots. Only
	// what goes in it afterwards is a medium change. No pool: an empty drive has no volume to keep.
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

	// A medium is copied into a volume of the drive's pool, so loading one names a pool.
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

		// Read before the guest is asked anything: a domain which was replaced has already taken
		// the console with it, and that error would otherwise be the only thing reported.
		suite.Assert().Equal(started, suite.domainID(node, name),
			"libvirt replaced the domain instead of changing the medium to %q", medium)
		suite.Assert().NoError(suite.guestReadsMedium(console, step.nonce),
			"the guest did not read %q", medium)
	}

	// The guest holds the drive open, and Linux locks the tray of a cdrom it has open. The change is
	// refused rather than forced, and retried until the guest lets go.
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

	// Ejected back to a drive naming no pool, as it started.
	suite.ejectMedium(nodeCtx, doc, "", loaded.source)
	suite.Assert().Equal(started, suite.domainID(node, name), "libvirt replaced the domain instead of ejecting the medium")
	suite.Assert().NoError(suite.guestTrayIsEmpty(console), "the guest still finds a medium after the eject")

	if suite.T().Failed() {
		return
	}

	suite.detachAlpineConsole(console)

	// The control: memory is not hot-pluggable, so this edit replaces the domain. Were it to keep the
	// ID as well, an unchanged ID above would prove nothing.
	doc.MemoryConfig.MemorySize = meta.MustByteSize(strconv.FormatUint(640<<20, 10))
	suite.replaceVirtualMachineConfig(nodeCtx, doc)
	suite.Assert().Eventually(func() bool {
		id, ok := suite.tryDomainID(node, name)

		return ok && id != started
	}, 2*time.Minute, time.Second, "a memory change did not replace the domain")
}

// TestCDROMMediaChangeWhileAProfileEditIsPending loads a medium in the same edit that switches the
// hardware profile of a running guest. The medium goes in at once and the guest keeps running on the
// profile it started with; the new profile is applied at the next stop/start, with the medium still
// in the drive.
//
//nolint:gocyclo
func (suite *LibvirtSuite) TestCDROMMediaChangeWhileAProfileEditIsPending() {
	node, nodeCtx := suite.requireAlpineGuestNode()

	doc := suite.prepareAlpineGuestOnNode(node, nodeCtx, nil, 512<<20, "host")
	name := doc.Name()

	library := doc.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageLibrary
	pool := doc.DisksConfig[0].DiskPool

	mediumDigest, mediumNonce := suite.uploadMedium(nodeCtx, library, "media-a.iso")

	recorder := suite.recordDomainSpec(nodeCtx, name)
	defer recorder.stop()

	suite.addEmptyMediaDrive(nodeCtx, doc)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 2, 0)

	started := suite.domainID(node, name)
	startedCPU := suite.liveCPUMode(node, name)
	suite.Require().NotEqual("host-passthrough", startedCPU)

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

	// One edit carries both: the profile switch and the medium.
	passthrough := "passthrough"
	doc.ProfileConfig = &passthrough

	source := suite.loadMedium(nodeCtx, doc, pool, library, "media-a.iso", mediumDigest)

	suite.Assert().Equal(started, suite.domainID(node, name), "the medium change applied the pending profile by restarting")
	suite.Assert().Equal(startedCPU, suite.liveCPUMode(node, name), "the running guest changed its CPU without a restart")
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

	suite.Assert().Equal("host-passthrough", suite.liveCPUMode(node, name), "the pending profile was not applied at the restart")
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
	// lockedTrayHold is how long a change refused by a locked tray is watched for not going through:
	// longer than one attempt to change the medium, and a retry after it.
	lockedTrayHold = 45 * time.Second
	// mediaRetryWait bounds how long a refused change takes to go through once the guest lets go.
	mediaRetryWait = 2 * time.Minute
)

// addEmptyMediaDrive appends the media drive to the virtual machine, empty and naming no pool.
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

// assertEmptyDriveReady waits for the empty media drive to be reported ready, backed by no pool.
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

// assertMediumWithoutPoolRejected loads a medium into the media drive without naming a pool, and
// expects the node to refuse the configuration. The document is left as it was.
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

// tryDomainID reads the ID of the running domain, reporting false while there is none.
func (suite *LibvirtSuite) tryDomainID(node, name string) (string, bool) {
	text, code := suite.RunDebugContainer(suite.ctx, node, "/usr/local/bin/virsh", "--connect", libvirtURI, "dominfo", name)
	if code != 0 {
		return "", false
	}

	match := domainID.FindStringSubmatch(text)
	if len(match) != 2 {
		return "", false
	}

	return match[1], true
}

// liveDefinitionReadsEventually reports whether the running domain reads source, waiting up to
// within for it to. A zero within checks once.
//
// Polled rather than asserted: the caller may want either answer, and a failed read is no answer.
func (suite *LibvirtSuite) liveDefinitionReadsEventually(node, name, source string, within time.Duration) bool {
	deadline := time.Now().Add(within)

	for {
		text, code := suite.RunDebugContainer(suite.ctx, node, "/usr/local/bin/virsh", "--connect", libvirtURI, "dumpxml", name)
		if code == 0 && strings.Contains(text, source) {
			return true
		}

		if !time.Now().Before(deadline) {
			return false
		}

		time.Sleep(2 * time.Second)
	}
}

// liveCPUMode reads the CPU mode of the running domain. Libvirt expands host-model into a custom
// model in a running definition, so a guest started on host-model reads custom.
func (suite *LibvirtSuite) liveCPUMode(node, name string) string {
	suite.T().Helper()

	var domain libvirtxml.Domain
	suite.Require().NoError(domain.Unmarshal(suite.runVirsh(node, "dumpxml", name)))

	if domain.CPU == nil {
		return ""
	}

	return domain.CPU.Mode
}

// guestLockTray holds the media drive open in the guest shell. Linux locks the tray of a cdrom it
// has open, and the lock lasts until the descriptor is closed.
func (suite *LibvirtSuite) guestLockTray(console *alpineConsole) error {
	suite.T().Helper()

	return suite.runGuestProbe(console, fmt.Sprintf("exec 3<%s", mediaDevice),
		"TRAY_LOCKED_"+strings.ReplaceAll(uuid.NewString(), "-", ""))
}

// guestUnlockTray closes the descriptor guestLockTray opened, and with it unlocks the tray.
func (suite *LibvirtSuite) guestUnlockTray(console *alpineConsole) error {
	suite.T().Helper()

	return suite.runGuestProbe(console, "exec 3<&-",
		"TRAY_UNLOCKED_"+strings.ReplaceAll(uuid.NewString(), "-", ""))
}

// uploadMedium puts one sector of recognizable bytes into the library and reports its digest and
// the nonce a guest reading the drive will see.
func (suite *LibvirtSuite) uploadMedium(nodeCtx context.Context, library, file string) (string, string) {
	suite.T().Helper()

	nonce := strings.ReplaceAll(uuid.NewString(), "-", "")

	contents := make([]byte, mediaSectorSize)
	copy(contents, "MEDIUM_"+nonce)

	// Pinned, so the node hashes the file rather than taking it on trust, both on the way in and
	// again whenever the drive resolves against it.
	pinned := digest.FromBytes(contents).String()

	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, file, pinned, bytes.NewReader(contents))
	suite.Require().NoError(err)

	return pinned, nonce
}

// loadMedium points the drive at a library file and waits for the definition to say so. It returns
// the path the definition reads the medium from.
//
// The definition does not name the library file: the medium is copied into the drive's pool and the
// domain reads the copy. So the path is taken from the disk status published for this medium, whose
// ID covers the file and digest, and only then looked for in the definition.
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

// ejectMedium takes the medium read from source out of the drive, leaving the drive itself where it
// is, naming pool -- which an empty drive may leave empty.
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

// setMediaProvision rewrites where the media drive's contents come from and the pool it names, and
// returns the drive as applied.
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

// replaceVirtualMachineConfig applies doc in place of the document of the same name.
//
// Replacement rather than a patch: a disk's source is exactly one of blank or an image, and a patch
// merges field by field, so it can add the new source without taking the old one away.
func (suite *LibvirtSuite) replaceVirtualMachineConfig(nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) {
	suite.T().Helper()

	suite.Require().NoError(suite.applyVirtualMachineConfig(nodeCtx, doc))
}

// applyVirtualMachineConfig is replaceVirtualMachineConfig reporting what the node made of the
// configuration rather than requiring it to be accepted.
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

// domainID reads the ID libvirt assigned the running domain. It changes when the domain is
// replaced, and only then.
func (suite *LibvirtSuite) domainID(node, name string) string {
	suite.T().Helper()

	text, code := suite.RunDebugContainer(suite.ctx, node, "/usr/local/bin/virsh", "--connect", libvirtURI, "dominfo", name)
	suite.Require().Zero(code, "virsh dominfo %q failed: %s", name, text)

	match := domainID.FindStringSubmatch(text)
	suite.Require().Len(match, 2, "virsh dominfo %q reported no ID: %s", name, text)

	return match[1]
}

// guestReadsMedium waits for the guest to see the medium now in the drive.
//
// Retried in the guest rather than asserted once: a medium change reaches the guest as a unit
// attention its kernel acts on asynchronously, so the first read after one can still fail.
func (suite *LibvirtSuite) guestReadsMedium(console *alpineConsole, nonce string) error {
	suite.T().Helper()

	return suite.runGuestProbe(console,
		fmt.Sprintf("dd if=%s bs=%d count=1 2>/dev/null | tr -d '\\0' | grep -q MEDIUM_%s", mediaDevice, mediaSectorSize, nonce),
		"MEDIUM_READ_"+nonce)
}

// guestTrayIsEmpty waits for the guest to find no medium in the drive.
func (suite *LibvirtSuite) guestTrayIsEmpty(console *alpineConsole) error {
	suite.T().Helper()

	return suite.runGuestProbe(console,
		fmt.Sprintf("! dd if=%s bs=%d count=1 >/dev/null 2>&1", mediaDevice, mediaSectorSize),
		"MEDIUM_ABSENT_"+strings.ReplaceAll(uuid.NewString(), "-", ""))
}

// runGuestProbe retries one shell test in the guest until it holds, and reports a marker the
// terminal's own echo cannot produce: the shell prints it, and it is assembled on the wire.
//
// Reports rather than fails: a console which has gone away is itself evidence about the domain, and
// the caller has other evidence to collect before deciding what the run means.
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

// domainSpecRecorder keeps every definition a virtual machine was given while a case ran.
//
// A guest is taken down between two polls and brought back up before the next one, so polling says
// nothing about whether it ever went away. Watching does: the states are reported in order, and the
// one that only existed for a moment is reported with the rest.
type domainSpecRecorder struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu          sync.Mutex
	transitions []string
}

// recordDomainSpec starts recording the domain spec and the observed domain of one virtual machine.
func (suite *LibvirtSuite) recordDomainSpec(nodeCtx context.Context, name string) *domainSpecRecorder {
	suite.T().Helper()

	ctx, cancel := context.WithCancel(nodeCtx)
	recorder := &domainSpecRecorder{cancel: cancel, done: make(chan struct{})}

	events := make(chan state.Event)

	// A single-resource watch opens with the state in force, so the definition recording started
	// against is the first entry and every later one reads as a change from something.
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

// record describes one event in a single line, keeping only what tells a replacement from a change:
// the power intent, the identity of the definition, and whether the domain was observed at all.
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

// shortDigest names a definition without printing it. Two entries naming the same digest are the
// same definition; a definition which changed at all gets a new one.
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
