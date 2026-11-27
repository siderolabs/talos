// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// VideoProbeSuite collects diagnostics on video devices and screenshots for each architecture,
// to settle how arm64 guests should present a display.
//
// The probes log their findings with a "video-probe:" prefix and only fail on test infrastructure
// errors: what a model or Talos renders on a given architecture is the result, not a failure.
// Run with -test.v and grep the log for "video-probe:".
type VideoProbeSuite struct{ LibvirtSuite }

// SuiteName implements base.NamedSuite.
func (suite *VideoProbeSuite) SuiteName() string { return "hypervisor.VideoProbeSuite" }

// SetupTest skips the suite in short mode: each probe boots guests.
func (suite *VideoProbeSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping video probes in short mode")
	}

	suite.LibvirtSuite.SetupTest()
}

const (
	probeBusybox = "/nix/var/nix/profiles/default/bin/busybox"
	probeShell   = "/nix/var/nix/profiles/default/bin/sh"

	// probeShots screenshots are taken probeShotInterval apart, so firmware has time to draw.
	probeShots         = 15
	probeShotInterval  = 2 * time.Second
	probeBrightnessMin = 16
)

func (suite *VideoProbeSuite) logf(format string, args ...any) {
	suite.T().Helper()
	suite.T().Logf("video-probe: "+format, args...)
}

// TestProbeCapabilities logs what libvirt and QEMU on the node offer for video and input.
func (suite *VideoProbeSuite) TestProbeCapabilities() {
	node := suite.RandomDiscoveredNodeInternalIP()
	arch := suite.guestArch(node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	suite.logf("node %s arch %s firmware %s", node, arch.qemu, arch.firmware)

	seen := map[string]bool{}

	for _, profile := range arch.profiles {
		if seen[profile.machine] {
			continue
		}

		seen[profile.machine] = true

		args := []string{"/usr/local/bin/virsh", "--connect", libvirtURI, "domcapabilities", "--machine", profile.machine, "--virttype", "kvm"}

		output, code := suite.RunDebugContainer(suite.ctx, node, args...)
		if code != 0 {
			suite.logf("domcapabilities machine=%s failed (exit %d): %s", profile.machine, code, strings.TrimSpace(output))

			continue
		}

		var caps struct {
			Machine string `xml:"machine"`
			Devices struct {
				Video    probeEnums `xml:"video"`
				Graphics probeEnums `xml:"graphics"`
				Input    probeEnums `xml:"input"`
			} `xml:"devices"`
		}

		if err := xml.Unmarshal([]byte(output), &caps); err != nil {
			suite.logf("domcapabilities machine=%s unparsable: %v", profile.machine, err)

			continue
		}

		suite.logf("domcapabilities machine=%s (resolved %s): video %s; graphics %s; input %s",
			profile.machine, caps.Machine, caps.Devices.Video, caps.Devices.Graphics, caps.Devices.Input)
	}

	// Print QEMU's exit code and, should nothing match, the head of its output, so a missing binary
	// or a changed format shows up rather than an empty list.
	script := fmt.Sprintf(`out=$(/usr/local/bin/qemu-system-%[1]s -device help 2>&1); echo "qemu exit:$?"
echo "$out" | %[2]s grep -iE 'gpu|vga|ramfb|bochs|cirrus|display|tablet|keyboard|xhci|virtio-input' || echo "$out" | %[2]s head -n 20`,
		arch.qemu, probeBusybox)

	output, code := suite.RunDebugContainer(suite.ctx, node, probeShell, "-c", script)
	suite.logf("qemu-system-%s -device help (script exit %d), display and input devices:\n%s", arch.qemu, code, strings.TrimSpace(output))
}

// TestProbeTalosDomains logs what Talos-managed guests get on this architecture, with VNC (where
// arm64 relies on libvirt's implicit video device) and with `video.enabled` alone, and what
// screenshots of them show while firmware boots.
func (suite *VideoProbeSuite) TestProbeTalosDomains() {
	node := suite.RandomDiscoveredNodeInternalIP()
	arch := suite.guestArch(node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	summary := make([]string, 0, 2)

	for _, tt := range []struct {
		label string
		vnc   bool
		video *bool
	}{
		{label: "talos-vnc", vnc: true},
		{label: "talos-video-only", video: new(true)},
	} {
		name := suite.probeTalosVM(node, arch, tt.vnc, tt.video)

		output, code := suite.RunDebugContainer(suite.ctx, node, probeShell, "-c", probeScript(name, ""))
		suite.Require().EqualValues(0, code, "video probe script failed for %s: %s", tt.label, output)

		summary = append(summary, tt.label+": "+suite.parseProbeOutput(arch.qemu, tt.label, output))
	}

	suite.logf("arch %s talos summary:\n  %s", arch.qemu, strings.Join(summary, "\n  "))
}

// probeTalosVM starts a diskless Talos-managed guest, whose firmware keeps drawing to the display.
func (suite *VideoProbeSuite) probeTalosVM(node string, arch guestArch, vnc bool, video *bool) string {
	suite.T().Helper()

	nodeCtx := client.WithNode(suite.ctx, node)
	name := "vm-video-probe-" + uuid.NewString()

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = arch.firmware
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("256MiB")
	doc.ConsoleConfig.VNCConfig.VNCEnabled = new(vnc)
	doc.VideoConfig.VideoEnabled = video

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertVirtualMachineStatus(nodeCtx, name, hypervisorhelpers.VirtualMachinePowerStateRunning)

	return name
}

// TestProbeVideoModels starts a bare libvirt domain per video model, without VNC, and logs whether
// libvirt accepts the model, the devices it renders, and what screenshots show while firmware boots.
//
// The domains are created directly through libvirt and are not Talos-owned: they exercise libvirt
// and QEMU without depending on what the Talos platform code renders.
func (suite *VideoProbeSuite) TestProbeVideoModels() {
	node := suite.RandomDiscoveredNodeInternalIP()
	arch := suite.guestArch(node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	models := []string{"virtio", "ramfb", "vga", "bochs"}
	if arch.qemu == constants.KernelArchAmd64 {
		models = append(models, "cirrus")
	}

	summary := make([]string, 0, len(models))

	for _, model := range models {
		summary = append(summary, model+": "+suite.probeVideoModel(node, arch, model))
	}

	suite.logf("arch %s summary:\n  %s", arch.qemu, strings.Join(summary, "\n  "))
}

func (suite *VideoProbeSuite) probeVideoModel(node string, arch guestArch, model string) string {
	name := "talos-video-probe-" + model + "-" + uuid.NewString()[:8]
	machineType := arch.profile("host").machine

	osFirmware := ""
	cpu := ""

	if arch.qemu == constants.KernelArchArm64 {
		// Mirrors what Talos renders for arm64 guests: UEFI and the host CPU under KVM.
		osFirmware = ` firmware="efi"`
		cpu = `<cpu mode="host-passthrough"/>`
	}

	domainXML := fmt.Sprintf(`<domain type="kvm">
  <name>%s</name>
  <memory unit="MiB">256</memory>
  <vcpu placement="static">1</vcpu>
  <os%s>
    <type arch="%s" machine="%s">hvm</type>
  </os>
  %s
  <devices>
    <emulator>/usr/local/bin/qemu-system-%s</emulator>
    <video><model type="%s"/></video>
  </devices>
</domain>
`, name, osFirmware, arch.qemu, machineType, cpu, arch.qemu, model)

	// The script destroys the domain itself; this covers it being interrupted. The suite context
	// is canceled by the time cleanups run.
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		suite.RunDebugContainer(ctx, node, "/usr/local/bin/virsh", "--connect", libvirtURI, "destroy", name)
	})

	script := probeScript(name, domainXML)

	output, code := suite.RunDebugContainer(suite.ctx, node, probeShell, "-c", script)
	suite.Require().EqualValues(0, code, "video probe script failed for model %s: %s", model, output)

	return suite.parseProbeOutput(arch.qemu, model, output)
}

// probeScript dumps the live definition of a domain and takes probeShots screenshots of it with
// virsh, printing one tagged line per result for parseProbeOutput. With domainXML, it first creates
// the domain as a transient one and destroys it afterwards; otherwise the domain must be running.
func probeScript(name, domainXML string) string {
	// $V is word-split, so quotes would be passed to virsh literally; set -f keeps the URI's "?"
	// from being globbed instead.
	virsh := "/usr/local/bin/virsh --connect " + libvirtURI

	create := `echo "CREATE:0:existing domain"`
	destroy := ""

	if domainXML != "" {
		create = fmt.Sprintf(`echo %s | $B base64 -d > /tmp/probe.xml
out=$($V create /tmp/probe.xml 2>&1); rc=$?
echo "CREATE:$rc:$(echo "$out" | oneline)"
[ $rc -eq 0 ] || exit 0`, base64.StdEncoding.EncodeToString([]byte(domainXML)))
		destroy = fmt.Sprintf(`$V destroy %s >/dev/null 2>&1`, name)
	}

	return fmt.Sprintf(`
set -f
B=%[1]s
V="%[2]s"
oneline() { $B tr '\n' ' '; }
%[3]s
echo "LIVEXML:$($V dumpxml %[4]s | $B base64 -w0)"
i=0
while [ $i -lt %[5]d ]; do
  out=$($V screenshot %[4]s /tmp/probe.png 2>&1); rc=$?
  echo "SHOT:$i:$rc:$(echo "$out" | oneline)"
  [ $rc -eq 0 ] && echo "PNG:$i:$($B base64 -w0 /tmp/probe.png)"
  i=$((i+1))
  $B sleep %[6]d
done
%[7]s
exit 0
`, probeBusybox, virsh, create, name, probeShots, int(probeShotInterval.Seconds()), destroy)
}

var probeLine = regexp.MustCompile(`^(CREATE|LIVEXML|SHOT|PNG):(.*)$`)

//nolint:gocyclo // One branch per probe output line kind.
func (suite *VideoProbeSuite) parseProbeOutput(arch, model, output string) string {
	var (
		result    = "no output"
		firstLit  = -1
		shotsOK   int
		shotsFail int
		lastError string
	)

	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 0, 1<<20), 64<<20)

	for scanner.Scan() {
		match := probeLine.FindStringSubmatch(scanner.Text())
		if match == nil {
			continue
		}

		switch match[1] {
		case "CREATE":
			rc, message, _ := strings.Cut(match[2], ":")
			suite.logf("arch %s model %s: create exit %s: %s", arch, model, rc, strings.TrimSpace(message))

			if rc != "0" {
				return "create failed: " + strings.TrimSpace(message)
			}

			result = "created"
		case "LIVEXML":
			text, err := base64.StdEncoding.DecodeString(match[2])
			if err != nil {
				suite.logf("arch %s model %s: live XML undecodable: %v", arch, model, err)

				continue
			}

			suite.logLiveDevices(arch, "model "+model, string(text))
		case "SHOT":
			fields := strings.SplitN(match[2], ":", 3)
			if len(fields) == 3 && fields[1] != "0" {
				shotsFail++
				lastError = strings.TrimSpace(fields[2])
				suite.logf("arch %s model %s shot %s: virsh exit %s: %s", arch, model, fields[0], fields[1], lastError)
			}
		case "PNG":
			index, data, _ := strings.Cut(match[2], ":")

			raw, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				suite.logf("arch %s model %s shot %s: undecodable base64: %v", arch, model, index, err)

				continue
			}

			shotsOK++

			stats := imageStats(raw)
			suite.logf("arch %s model %s shot %s: %d bytes, %s", arch, model, index, len(raw), stats)

			if shot, err := strconv.Atoi(index); err == nil && firstLit < 0 && stats.lit > 0 {
				firstLit = shot
			}
		}
	}

	if result == "created" {
		result = fmt.Sprintf("created, %d screenshots ok, %d failed", shotsOK, shotsFail)

		if firstLit >= 0 {
			result += fmt.Sprintf(", first non-black at shot %d (~%s)", firstLit, time.Duration(firstLit)*probeShotInterval)
		} else if shotsOK > 0 {
			result += ", never non-black"
		}

		if lastError != "" {
			result += ", last error: " + lastError
		}
	}

	return result
}

// logLiveDevices logs the display-related devices libvirt rendered for a live domain.
func (suite *VideoProbeSuite) logLiveDevices(arch, label, text string) {
	suite.T().Helper()

	var domain libvirtxml.Domain
	if err := domain.Unmarshal(text); err != nil || domain.Devices == nil {
		suite.logf("arch %s %s: live XML unparsable: %v", arch, label, err)

		return
	}

	machineType := ""
	if domain.OS != nil && domain.OS.Type != nil {
		machineType = domain.OS.Type.Machine
	}

	suite.logf("arch %s %s: machine %s; video %v; graphics %v; input %v; usb controllers %v",
		arch, label, machineType, probeVideos(domain.Devices), probeGraphics(domain.Devices),
		probeInputs(domain.Devices), probeUSBControllers(domain.Devices))
}

func probeVideos(devices *libvirtxml.DomainDeviceList) []string {
	videos := make([]string, 0, len(devices.Videos))

	for _, video := range devices.Videos {
		entry := video.Model.Type
		if video.Model.VRam > 0 {
			entry += fmt.Sprintf(" vram=%dKiB", video.Model.VRam)
		}

		if video.Model.Primary != "" {
			entry += " primary=" + video.Model.Primary
		}

		videos = append(videos, entry)
	}

	return videos
}

func probeGraphics(devices *libvirtxml.DomainDeviceList) []string {
	graphics := make([]string, 0, len(devices.Graphics))

	for _, graphic := range devices.Graphics {
		if graphic.VNC != nil {
			graphics = append(graphics, "vnc")
		} else {
			graphics = append(graphics, "other")
		}
	}

	return graphics
}

func probeInputs(devices *libvirtxml.DomainDeviceList) []string {
	inputs := make([]string, 0, len(devices.Inputs))

	for _, input := range devices.Inputs {
		inputs = append(inputs, input.Type+"/"+input.Bus)
	}

	return inputs
}

func probeUSBControllers(devices *libvirtxml.DomainDeviceList) []string {
	var controllers []string

	for _, controller := range devices.Controllers {
		if controller.Type == "usb" {
			controllers = append(controllers, "usb/"+controller.Model)
		}
	}

	return controllers
}

type probeEnums struct {
	Supported string `xml:"supported,attr"`
	Enums     []struct {
		Name   string   `xml:"name,attr"`
		Values []string `xml:"value"`
	} `xml:"enum"`
}

func (e probeEnums) String() string {
	parts := make([]string, 0, 1+len(e.Enums))
	parts = append(parts, "supported="+e.Supported)

	for _, enum := range e.Enums {
		parts = append(parts, enum.Name+"="+strings.Join(enum.Values, ","))
	}

	return strings.Join(parts, " ")
}

type probeImageStats struct {
	err           error
	width, height int
	lit           int
	total         int
	colors        int
}

func (s probeImageStats) String() string {
	if s.err != nil {
		return "undecodable: " + s.err.Error()
	}

	return fmt.Sprintf("%dx%d, %.2f%% non-black, %d colors (capped at 256)",
		s.width, s.height, 100*float64(s.lit)/float64(max(s.total, 1)), s.colors)
}

// imageStats decodes a PNG and measures how much of it is drawn: a display nothing has drawn to
// is black, while firmware or a console leave text or a logo.
func imageStats(raw []byte) probeImageStats {
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return probeImageStats{err: err}
	}

	return measure(img)
}

func measure(img image.Image) probeImageStats {
	bounds := img.Bounds()
	stats := probeImageStats{width: bounds.Dx(), height: bounds.Dy()}
	colors := make(map[uint32]struct{}, 256)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r, g, b = r>>8, g>>8, b>>8

			stats.total++

			if r > probeBrightnessMin || g > probeBrightnessMin || b > probeBrightnessMin {
				stats.lit++
			}

			if len(colors) < 256 {
				colors[r<<16|g<<8|b] = struct{}{}
			}
		}
	}

	stats.colors = len(colors)

	return stats
}

func init() {
	allSuites = append(allSuites, new(VideoProbeSuite))
}
