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
	"strings"
	"sync"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
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
// The proof is in two halves which have to agree: libvirt still reports the ID it assigned when the
// domain started, and the guest -- which would have lost its uptime to a restart -- reads each new
// medium as it arrives.
//
// The two are checked in that order and neither aborts the other. A guest which lost its console
// says nothing about why on its own, so the domain identity is read first and every observation is
// collected before the case is allowed to fail: a replaced domain and a dropped console look alike
// from the console end, and only the recorded definition says which happened.
//
//nolint:gocyclo
func (suite *LibvirtSuite) TestCDROMMediaChangeKeepsTheGuestRunning() {
	node, nodeCtx := suite.requireAlpineGuestNode()

	// No DHCP reservation: this case needs a booted guest with a console, not a network.
	doc := suite.prepareAlpineGuestOnNode(node, nodeCtx, nil, 512<<20)
	name := doc.Name()

	library := doc.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageLibrary

	firstDigest, firstNonce := suite.uploadMedium(nodeCtx, library, "media-a.iso")
	secondDigest, secondNonce := suite.uploadMedium(nodeCtx, library, "media-b.iso")

	// Started before the first edit, so every definition the node went through while the medium
	// moved is on the record, including one it held for less time than anything here could poll.
	recorder := suite.recordDomainSpec(nodeCtx, name)
	defer recorder.stop()

	// The drive itself is a device, so adding it redefines the domain and the guest reboots. Only
	// what goes in it afterwards is a medium change.
	doc.DisksConfig = append(doc.DisksConfig, hypervisorcfg.VirtualMachineDisk{
		DiskName: mediaDiskName,
		DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
		ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
			BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{},
		},
	})

	suite.replaceVirtualMachineConfig(nodeCtx, doc)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 2, 0)

	started := suite.domainID(node, name)

	console := suite.loginAlpineConsole(nodeCtx, name)
	defer func() {
		if suite.T().Failed() {
			suite.T().Logf("bounded Alpine console transcript: %q", console.transcript)
			recorder.report(suite.T())
		}
	}()

	suite.Assert().NoError(suite.guestTrayIsEmpty(console), "the drive starts out empty")

	for _, step := range []struct {
		medium string
		digest string
		nonce  string
	}{
		{medium: "media-a.iso", digest: firstDigest, nonce: firstNonce},
		{medium: "media-b.iso", digest: secondDigest, nonce: secondNonce},
	} {
		suite.loadMedium(nodeCtx, doc, library, step.medium, step.digest)

		// Read before the guest is asked anything: a domain which was replaced has already taken
		// the console with it, and that error would otherwise be the only thing reported.
		suite.Assert().Equal(started, suite.domainID(node, name),
			"libvirt replaced the domain instead of changing the medium to %q", step.medium)
		suite.Assert().NoError(suite.guestReadsMedium(console, step.nonce),
			"the guest did not read %q", step.medium)
	}

	suite.ejectMedium(nodeCtx, doc)
	suite.Assert().Equal(started, suite.domainID(node, name), "libvirt replaced the domain instead of ejecting the medium")
	suite.Assert().NoError(suite.guestTrayIsEmpty(console), "the guest still finds a medium after the eject")

	if suite.T().Failed() {
		return
	}

	suite.detachAlpineConsole(console)
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

	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, file, false, pinned, bytes.NewReader(contents))
	suite.Require().NoError(err)

	return pinned, nonce
}

// loadMedium points the drive at a library file and waits for the definition to say so.
func (suite *LibvirtSuite) loadMedium(nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1, library, file, pinned string) {
	suite.T().Helper()

	suite.setMediaProvision(nodeCtx, doc, hypervisorcfg.VirtualMachineDiskProvision{
		FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
			ImageLibrary: library,
			ImageFile:    file,
			ImageDigest:  pinned,
		},
	})

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, doc.Name(),
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Contains(spec.TypedSpec().DomainXML, file)
		},
	)
}

// ejectMedium takes the medium out of the drive, leaving the drive itself where it is.
func (suite *LibvirtSuite) ejectMedium(nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) {
	suite.T().Helper()

	suite.setMediaProvision(nodeCtx, doc, hypervisorcfg.VirtualMachineDiskProvision{
		BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{},
	})

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, doc.Name(),
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.NotContains(spec.TypedSpec().DomainXML, "media-")
		},
	)
}

// setMediaProvision rewrites where the media drive's contents come from.
func (suite *LibvirtSuite) setMediaProvision(
	nodeCtx context.Context,
	doc *hypervisorcfg.VirtualMachineConfigV1Alpha1, provision hypervisorcfg.VirtualMachineDiskProvision,
) {
	suite.T().Helper()

	for i := range doc.DisksConfig {
		if doc.DisksConfig[i].DiskName == mediaDiskName {
			doc.DisksConfig[i].ProvisionConfig = provision

			suite.replaceVirtualMachineConfig(nodeCtx, doc)

			return
		}
	}

	suite.Require().FailNow("the virtual machine has no media drive to change")
}

// replaceVirtualMachineConfig applies doc in place of the document of the same name.
//
// Replacement rather than a patch: a disk's source is exactly one of blank or an image, and a patch
// merges field by field, so it can add the new source without taking the old one away.
func (suite *LibvirtSuite) replaceVirtualMachineConfig(nodeCtx context.Context, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) {
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
	suite.Require().NoError(err)
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
	suite.Require().NoError(suite.Client.COSI.Watch(ctx,
		hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name).Metadata(), events))
	suite.Require().NoError(suite.Client.COSI.Watch(ctx,
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
