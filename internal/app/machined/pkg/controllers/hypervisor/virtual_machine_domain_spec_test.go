// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"context"
	"embed"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/relaxng"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

//go:embed testdata/libvirt/*.rng
var libvirtSchemas embed.FS

var domainGrammar struct {
	sync.Once
	grammar *relaxng.Grammar
	err     error
}

func compileDomainGrammar(schemas fs.FS) (*relaxng.Grammar, error) {
	schema, err := fs.ReadFile(schemas, "domain.rng")
	if err != nil {
		return nil, fmt.Errorf("read libvirt domain schema: %w", err)
	}

	ctx := context.Background()

	parsed, err := helium.NewParser().Parse(ctx, schema)
	if err != nil {
		return nil, fmt.Errorf("parse libvirt domain schema: %w", err)
	}

	collector := helium.NewErrorCollector(ctx, helium.ErrorLevelNone)

	grammar, err := relaxng.NewCompiler().FS(schemas).BaseDir(".").ErrorHandler(collector).Compile(ctx, parsed)
	if err := errors.Join(err, errors.Join(collector.Errors()...)); err != nil {
		return nil, fmt.Errorf("compile libvirt domain schema: %w", err)
	}

	if grammar == nil {
		return nil, errors.New("compile libvirt domain schema: nil grammar")
	}

	return grammar, nil
}

var heliumDriverElement = regexp.MustCompile(`<driver[^>]*(></driver>|/>)`)

func validateDomainXML(data []byte) error {
	domainGrammar.Do(func() {
		schemas, err := fs.Sub(libvirtSchemas, "testdata/libvirt")
		if err != nil {
			domainGrammar.err = fmt.Errorf("open libvirt schemas: %w", err)

			return
		}

		domainGrammar.grammar, domainGrammar.err = compileDomainGrammar(schemas)
	})

	if domainGrammar.err != nil {
		return domainGrammar.err
	}

	doc, err := helium.NewParser().Parse(context.Background(), hoistInterfaceSources(heliumDriverElement.ReplaceAll(data, nil)))
	if err != nil {
		return fmt.Errorf("parse domain XML: %w", err)
	}

	return relaxng.NewValidator(domainGrammar.grammar).Validate(context.Background(), doc)
}

var interfaceSource = regexp.MustCompile(`(?s)(<interface[^>]*>)(.*?)(\s*<source[^>]*>(?:</source>)?)`)

// hoistInterfaceSources moves each interface's source element to be its first child.
//
// The domain schema interleaves the source with an inner interleave holding mac, model and
// alias. helium fails to match an outer element placed between two inner ones, which is exactly
// the order libvirtxml emits (mac, source, model), even though the document is valid; xmllint
// accepts it. Interleave makes child order irrelevant, so validating the reordered document
// checks the same content.
func hoistInterfaceSources(data []byte) []byte {
	return interfaceSource.ReplaceAll(data, []byte("${1}${3}${2}"))
}

func TestDomainSchemaMissingInclude(t *testing.T) {
	t.Parallel()

	schemas := fstest.MapFS{
		"domain.rng": &fstest.MapFile{Data: []byte(`<grammar xmlns="http://relaxng.org/ns/structure/1.0"><include href="missing.rng"/></grammar>`)},
	}

	_, err := compileDomainGrammar(schemas)
	require.ErrorContains(t, err, "missing.rng")
}

func TestDomainSchemaValidation(t *testing.T) {
	t.Parallel()

	fixtures, err := filepath.Glob(filepath.Join("testdata", "virtualmachinespec", "*.xml"))
	require.NoError(t, err)
	require.NotEmpty(t, fixtures)

	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			t.Parallel()

			valid, err := os.ReadFile(fixture)
			require.NoError(t, err)
			require.NoError(t, validateDomainXML(valid))
		})
	}

	require.Error(t, validateDomainXML([]byte(`<domain type="kvm"><name>x</name><not-libvirt/></domain>`)))
	require.Error(t, validateDomainXML([]byte(`<domain>`)))
}

func (suite *VirtualMachineSpecSuite) TestExternalProducerPowerOverrideProjectsToDomain() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "external-override")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
	}
	suite.Create(spec)
	ctest.AssertResource(suite, spec.Metadata().ID(), func(domain *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", domain.TypedSpec().PowerState)
	})

	override := hypervisor.NewVirtualMachinePowerOverride(hypervisor.NamespaceName, spec.Metadata().ID())
	override.TypedSpec().PowerState = "stopped"
	override.TypedSpec().StopRequest = "graceful/request-1"
	suite.Create(override)
	ctest.AssertResource(suite, spec.Metadata().ID(), func(domain *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", domain.TypedSpec().PowerState)
		asrt.Equal("graceful/request-1", domain.TypedSpec().StopRequest)
	})

	current, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), spec.Metadata().ID())
	suite.Require().NoError(err)
	suite.Require().Equal("running", current.TypedSpec().PowerState)

	ctest.UpdateWithConflicts(suite, current, func(updated *hypervisor.VirtualMachineSpec) error {
		updated.TypedSpec().CPU.Count = 2

		return nil
	})
	ctest.AssertResource(suite, spec.Metadata().ID(), func(projected *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		var rendered libvirtxml.Domain
		if !asrt.NoError(rendered.Unmarshal(projected.TypedSpec().DomainXML)) {
			return
		}

		asrt.Equal(&libvirtxml.DomainVCPU{Value: 2}, rendered.VCPU)
		asrt.Equal("stopped", projected.TypedSpec().PowerState)
		asrt.Equal("graceful/request-1", projected.TypedSpec().StopRequest)
	})

	ctest.UpdateWithConflicts(suite, current, func(updated *hypervisor.VirtualMachineSpec) error {
		updated.TypedSpec().Profile = "missing"

		return nil
	})
	ctest.AssertResource(suite, spec.Metadata().ID(), func(projected *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", projected.TypedSpec().PowerState)
		asrt.Equal("forced", projected.TypedSpec().StopMode)
		asrt.Equal("graceful/request-1", projected.TypedSpec().StopRequest)
		asrt.NotEmpty(projected.TypedSpec().DomainXML)
	})

	ctest.UpdateWithConflicts(suite, override, func(updated *hypervisor.VirtualMachinePowerOverride) error {
		updated.TypedSpec().StopRequest = "graceful/request-2"

		return nil
	})
	ctest.AssertResource(suite, spec.Metadata().ID(), func(projected *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", projected.TypedSpec().PowerState)
		asrt.Equal("forced", projected.TypedSpec().StopMode)
		asrt.Equal("graceful/request-2", projected.TypedSpec().StopRequest)
	})
}

//nolint:gocyclo // Verify both architecture-specific defaults across console lifecycle transitions.
func (suite *VirtualMachineSpecSuite) TestVNCDevices() {
	for _, firmware := range []string{"bios", "uefi"} {
		for _, serial := range []bool{false, true} {
			name := fmt.Sprintf("vnc-%s-serial-%t", firmware, serial)
			spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
			*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
				CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
				Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
				PowerState: "stopped",
				Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: firmware},
				Console:    hypervisor.VirtualMachineConsoleSpec{Serial: serial},
			}
			suite.Create(spec)

			// Exercise enabling, disabling and re-enabling without leaving stale devices.
			for _, vnc := range []bool{false, true, false, true} {
				ctest.UpdateWithConflicts(suite, spec, func(current *hypervisor.VirtualMachineSpec) error {
					current.TypedSpec().Console.VNC = vnc

					return nil
				})
				ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
					var domain libvirtxml.Domain
					if !asrt.NoError(domain.Unmarshal(res.TypedSpec().DomainXML)) || !asrt.NotNil(domain.Devices) {
						return
					}

					if serial {
						if asrt.Len(domain.Devices.Serials, 1) {
							asrt.Equal(&libvirtxml.DomainChardevSource{
								Pty: &libvirtxml.DomainChardevSourcePty{},
							}, domain.Devices.Serials[0].Source)
						}
					} else {
						asrt.Empty(domain.Devices.Serials)
					}

					if vnc {
						if asrt.Len(domain.Devices.Graphics, 1) {
							// Exact equality excludes TCP, websocket and prescribed socket paths.
							asrt.Equal(&libvirtxml.DomainGraphicVNC{
								Listeners: []libvirtxml.DomainGraphicListener{{Socket: &libvirtxml.DomainGraphicListenerSocket{}}},
							}, domain.Devices.Graphics[0].VNC)
						}
					} else {
						asrt.Empty(domain.Devices.Graphics)
					}

					if vnc && runtime.GOARCH == "amd64" {
						if asrt.Len(domain.Devices.Videos, 1) {
							asrt.Equal("vga", domain.Devices.Videos[0].Model.Type)
						}

						if asrt.Len(domain.Devices.Controllers, 1) {
							asrt.Equal("usb", domain.Devices.Controllers[0].Type)
							asrt.Equal("qemu-xhci", domain.Devices.Controllers[0].Model)
						}

						if asrt.Len(domain.Devices.Inputs, 2) {
							asrt.Equal("tablet", domain.Devices.Inputs[0].Type)
							asrt.Equal("usb", domain.Devices.Inputs[0].Bus)
							asrt.Equal("keyboard", domain.Devices.Inputs[1].Type)
							asrt.Equal("usb", domain.Devices.Inputs[1].Bus)
						}
					} else {
						asrt.Empty(domain.Devices.Videos)
						asrt.Empty(domain.Devices.Controllers)
						asrt.Empty(domain.Devices.Inputs)
					}
				})

				rendered, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), name)
				suite.Require().NoError(err)
				suite.Require().NoError(validateDomainXML([]byte(rendered.TypedSpec().DomainXML)))
			}
		}
	}
}

func (suite *VirtualMachineSpecSuite) TestBuiltinProfilesRenderGuestHardware() {
	if runtime.GOARCH != "amd64" {
		suite.T().Skip("VM hardware profiles are only supported on amd64")
	}

	for _, tc := range []struct {
		name    string
		profile string
		machine string
		cpu     libvirtxml.DomainCPU
	}{
		{
			name:    "omitted",
			machine: "q35",
			cpu: libvirtxml.DomainCPU{
				Mode: "host-model",
			},
		},
		{
			name:    "host",
			profile: "host",
			machine: "q35",
			cpu: libvirtxml.DomainCPU{
				Mode: "host-model",
			},
		},
		{
			name:    "passthrough",
			profile: "passthrough",
			machine: "q35",
			cpu: libvirtxml.DomainCPU{
				Mode: "host-passthrough",
			},
		},
		{
			name:    "legacy",
			profile: "legacy",
			machine: "pc-i440fx-9.2",
			cpu: libvirtxml.DomainCPU{
				Mode:  "custom",
				Match: "exact",
				Model: &libvirtxml.DomainCPUModel{
					Value:    "kvm64",
					Fallback: "forbid",
				},
			},
		},
	} {
		for _, firmware := range []string{"bios", "uefi"} {
			suite.Run(tc.name+"-"+firmware, func() {
				name := "builtin-" + tc.name + "-" + firmware
				spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
				*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
					Profile: tc.profile,
					CPU: hypervisor.VirtualMachineCPUSpec{
						Count: 6,
						Topology: &hypervisor.VirtualMachineCPUTopologySpec{
							Sockets: 2,
							Cores:   3,
							Threads: 1,
						},
					},
					Memory: hypervisor.VirtualMachineMemorySpec{
						Size: 768 << 20,
					},
					PowerState: "stopped",
					Firmware: hypervisor.VirtualMachineFirmwareSpec{
						Type: firmware,
					},
				}
				suite.Create(spec)

				wantXML, err := os.ReadFile(filepath.Join("testdata", "virtualmachinespec", name+".xml"))
				suite.Require().NoError(err)

				expectedCPU := tc.cpu
				expectedCPU.XMLName = xml.Name{Local: "cpu"}
				expectedCPU.Topology = &libvirtxml.DomainCPUTopology{
					Sockets: 2,
					Cores:   3,
					Threads: 1,
				}

				ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
					var domain libvirtxml.Domain
					if !asrt.NoError(domain.Unmarshal(res.TypedSpec().DomainXML)) || !asrt.NotNil(domain.OS) {
						return
					}

					asrt.NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))
					asrt.Equal(&libvirtxml.DomainOSType{
						Type:    "hvm",
						Arch:    "x86_64",
						Machine: tc.machine,
					}, domain.OS.Type)
					asrt.Equal(string(wantXML), res.TypedSpec().DomainXML+"\n")
					asrt.Equal(&expectedCPU, domain.CPU)
					asrt.Equal(&libvirtxml.DomainVCPU{Value: 6}, domain.VCPU)
					asrt.Equal(&libvirtxml.DomainMemory{Value: 768 << 20, Unit: "bytes"}, domain.Memory)

					if firmware == "uefi" {
						asrt.Equal("efi", domain.OS.Firmware)
					} else {
						asrt.Empty(domain.OS.Firmware)
						asrt.Nil(domain.OS.Loader)
						asrt.Nil(domain.OS.NVRam)
					}
				})
			})
		}
	}
}

func (suite *VirtualMachineSpecSuite) TestCustomProfilesRenderGuestHardware() {
	if runtime.GOARCH != "amd64" {
		suite.T().Skip("VM hardware profiles are only supported on amd64")
	}

	for _, tc := range []struct {
		name    string
		machine string
		cpu     hypervisorcfg.VirtualMachineProfileCPU
		wantCPU libvirtxml.DomainCPU
	}{
		{
			name:    "host-model",
			machine: "pc-q35-9.2",
			cpu: hypervisorcfg.VirtualMachineProfileCPU{
				ModeConfig: hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
			},
			wantCPU: libvirtxml.DomainCPU{
				Mode: "host-model",
			},
		},
		{
			name:    "host-passthrough",
			machine: "pc",
			cpu: hypervisorcfg.VirtualMachineProfileCPU{
				ModeConfig: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
			},
			wantCPU: libvirtxml.DomainCPU{
				Mode: "host-passthrough",
			},
		},
		{
			name:    "default-fallback",
			machine: "q35",
			cpu: hypervisorcfg.VirtualMachineProfileCPU{
				ModeConfig:  hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
				ModelConfig: "EPYC-Rome-v1",
			},
			wantCPU: libvirtxml.DomainCPU{
				Mode:  "custom",
				Match: "exact",
				Model: &libvirtxml.DomainCPUModel{
					Value:    "EPYC-Rome-v1",
					Fallback: "forbid",
				},
			},
		},
		{
			name:    "forbid-fallback",
			machine: "pc-i440fx-9.2",
			cpu: hypervisorcfg.VirtualMachineProfileCPU{
				ModeConfig:     hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
				ModelConfig:    "Haswell-noTSX-IBRS",
				FallbackConfig: "forbid",
			},
			wantCPU: libvirtxml.DomainCPU{
				Mode:  "custom",
				Match: "exact",
				Model: &libvirtxml.DomainCPUModel{
					Value:    "Haswell-noTSX-IBRS",
					Fallback: "forbid",
				},
			},
		},
		{
			name:    "allow-fallback-and-features",
			machine: "pc-q35-9.2",
			cpu: hypervisorcfg.VirtualMachineProfileCPU{
				ModeConfig:     hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
				ModelConfig:    "Cascadelake-Server-v3",
				FallbackConfig: "allow",
				FeaturesConfig: []hypervisorcfg.VirtualMachineProfileCPUFeature{
					{
						NameConfig:   "aes",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyRequire,
					},
					{
						NameConfig:   "pku",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyDisable,
					},
					{
						NameConfig:   "avx2",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyOptional,
					},
					{
						NameConfig:   "hypervisor",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyForce,
					},
					{
						NameConfig:   "vmx",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyForbid,
					},
				},
			},
			wantCPU: libvirtxml.DomainCPU{
				Mode:  "custom",
				Match: "exact",
				Model: &libvirtxml.DomainCPUModel{
					Value:    "Cascadelake-Server-v3",
					Fallback: "allow",
				},
				Features: []libvirtxml.DomainCPUFeature{
					{
						Name:   "aes",
						Policy: "require",
					},
					{
						Name:   "pku",
						Policy: "disable",
					},
					{
						Name:   "avx2",
						Policy: "optional",
					},
					{
						Name:   "hypervisor",
						Policy: "force",
					},
					{
						Name:   "vmx",
						Policy: "forbid",
					},
				},
			},
		},
	} {
		for _, firmware := range []hypervisorhelpers.VirtualMachineFirmwareType{
			hypervisorhelpers.VirtualMachineFirmwareTypeBIOS,
			hypervisorhelpers.VirtualMachineFirmwareTypeUEFI,
		} {
			suite.Run(tc.name+"-"+firmware.String(), func() {
				name := "custom-" + tc.name + "-" + firmware.String()
				profile := hypervisorcfg.NewVirtualMachineProfileConfigV1Alpha1()
				profile.MetaName = name
				profile.MachineConfig = tc.machine
				profile.CPUConfig = tc.cpu

				vm := newVirtualMachine(name)
				vm.ProfileConfig = new(name)
				vm.FirmwareConfig.FirmwareType = firmware
				vm.CPUConfig.CPUCount = 6
				vm.CPUConfig.TopologyConfig.TopologySockets = new(uint32(2))
				vm.CPUConfig.TopologyConfig.TopologyCores = new(uint32(3))
				vm.CPUConfig.TopologyConfig.TopologyThreads = new(uint32(1))
				vm.MemoryConfig.MemorySize = meta.MustByteSize("768MiB")
				cfg, err := container.New(profile, vm)
				suite.Require().NoError(err)

				machine := config.NewMachineConfig(cfg)
				suite.Create(machine)

				wantXML, err := os.ReadFile(filepath.Join("testdata", "virtualmachinespec", name+".xml"))
				suite.Require().NoError(err)

				expectedCPU := tc.wantCPU
				expectedCPU.XMLName = xml.Name{Local: "cpu"}
				expectedCPU.Topology = &libvirtxml.DomainCPUTopology{
					Sockets: 2,
					Cores:   3,
					Threads: 1,
				}

				ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
					var domain libvirtxml.Domain
					if !asrt.NoError(domain.Unmarshal(res.TypedSpec().DomainXML)) || !asrt.NotNil(domain.OS) {
						return
					}

					asrt.NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))
					asrt.Equal(&libvirtxml.DomainOSType{
						Type:    "hvm",
						Arch:    "x86_64",
						Machine: tc.machine,
					}, domain.OS.Type)
					asrt.Equal(string(wantXML), res.TypedSpec().DomainXML+"\n")
					asrt.Equal(&expectedCPU, domain.CPU)
					asrt.Equal(&libvirtxml.DomainVCPU{Value: 6}, domain.VCPU)
					asrt.Equal(&libvirtxml.DomainMemory{Value: 768 << 20, Unit: "bytes"}, domain.Memory)

					if firmware == hypervisorhelpers.VirtualMachineFirmwareTypeUEFI {
						asrt.Equal("efi", domain.OS.Firmware)
					} else {
						asrt.Empty(domain.OS.Firmware)
					}
				})
				suite.Destroy(machine)
				ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, name)
			})
		}
	}
}

func (suite *VirtualMachineSpecSuite) TestProfileUpdatesReplaceGuestHardware() {
	if runtime.GOARCH != "amd64" {
		suite.T().Skip("VM hardware profiles are only supported on amd64")
	}

	profile := hypervisor.NewVirtualMachineProfileSpec(hypervisor.NamespaceName, "rack-a")
	*profile.TypedSpec() = hypervisor.VirtualMachineProfileSpecSpec{
		Machine: "pc-q35-9.2",
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode:     hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			Model:    "Haswell-noTSX-IBRS",
			Fallback: "allow",
			Features: []hypervisor.VirtualMachineProfileCPUFeatureSpec{
				{
					Name:   "aes",
					Policy: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyRequire,
				},
				{
					Name:   "pku",
					Policy: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyDisable,
				},
			},
		},
	}
	suite.Create(profile)

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "changing-profile")
	*vm.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		Profile: "rack-a",
		CPU: hypervisor.VirtualMachineCPUSpec{
			Count: 4,
			Topology: &hypervisor.VirtualMachineCPUTopologySpec{
				Sockets: 2,
				Cores:   1,
				Threads: 2,
			},
		},
		Memory: hypervisor.VirtualMachineMemorySpec{
			Size: 1 << 30,
		},
		PowerState: "running",
		Firmware: hypervisor.VirtualMachineFirmwareSpec{
			Type: "bios",
		},
	}
	suite.Create(vm)

	// This suite renders desired XML only; it does not start or restart live guests.
	assertRendered := func(machine string, cpu libvirtxml.DomainCPU) {
		suite.T().Helper()

		cpu.XMLName = xml.Name{Local: "cpu"}
		cpu.Topology = &libvirtxml.DomainCPUTopology{
			Sockets: 2,
			Cores:   1,
			Threads: 2,
		}

		ctest.AssertResource(suite, vm.Metadata().ID(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			var domain libvirtxml.Domain
			if !asrt.NoError(domain.Unmarshal(res.TypedSpec().DomainXML)) || !asrt.NotNil(domain.OS) || !asrt.NotNil(domain.OS.Type) {
				return
			}

			asrt.NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))
			asrt.Equal("running", res.TypedSpec().PowerState)
			asrt.Equal(machine, domain.OS.Type.Machine)
			asrt.Equal(&cpu, domain.CPU)
			asrt.Equal(&libvirtxml.DomainVCPU{Value: 4}, domain.VCPU)
			asrt.Equal(&libvirtxml.DomainMemory{Value: 1 << 30, Unit: "bytes"}, domain.Memory)
			asrt.Empty(domain.OS.Firmware)
		})
	}
	assertRendered("pc-q35-9.2", libvirtxml.DomainCPU{
		Mode:  "custom",
		Match: "exact",
		Model: &libvirtxml.DomainCPUModel{
			Value:    "Haswell-noTSX-IBRS",
			Fallback: "allow",
		},
		Features: []libvirtxml.DomainCPUFeature{
			{
				Name:   "aes",
				Policy: "require",
			},
			{
				Name:   "pku",
				Policy: "disable",
			},
		},
	})

	// Editing the referenced profile must replace the machine, model and feature
	// list even though the VM spec itself has not changed.
	before, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), vm.Metadata().ID())
	suite.Require().NoError(err)
	ctest.UpdateWithConflicts(suite, profile, func(current *hypervisor.VirtualMachineProfileSpec) error {
		*current.TypedSpec() = hypervisor.VirtualMachineProfileSpecSpec{
			Machine: "pc-i440fx-9.2",
			CPU: hypervisor.VirtualMachineProfileCPUSpec{
				Mode:  hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
				Model: "EPYC-Rome-v1",
				Features: []hypervisor.VirtualMachineProfileCPUFeatureSpec{
					{
						Name:   "pku",
						Policy: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyOptional,
					},
				},
			},
		}

		return nil
	})
	assertRendered("pc-i440fx-9.2", libvirtxml.DomainCPU{
		Mode:  "custom",
		Match: "exact",
		Model: &libvirtxml.DomainCPUModel{
			Value:    "EPYC-Rome-v1",
			Fallback: "forbid",
		},
		Features: []libvirtxml.DomainCPUFeature{
			{
				Name:   "pku",
				Policy: "optional",
			},
		},
	})

	after, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), vm.Metadata().ID())
	suite.Require().NoError(err)
	suite.Equal(before.Metadata().Version(), after.Metadata().Version())

	// Switching away from custom mode must remove model, match and features.
	ctest.UpdateWithConflicts(suite, profile, func(current *hypervisor.VirtualMachineProfileSpec) error {
		current.TypedSpec().Machine = "pc"
		current.TypedSpec().CPU = hypervisor.VirtualMachineProfileCPUSpec{
			Mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
		}

		return nil
	})
	assertRendered("pc", libvirtxml.DomainCPU{Mode: "host-passthrough"})

	// Selecting another profile replaces the old hardware in both directions.
	ctest.UpdateWithConflicts(suite, vm, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().Profile = "legacy"

		return nil
	})
	assertRendered("pc-i440fx-9.2", libvirtxml.DomainCPU{
		Mode:  "custom",
		Match: "exact",
		Model: &libvirtxml.DomainCPUModel{
			Value:    "kvm64",
			Fallback: "forbid",
		},
	})
	ctest.UpdateWithConflicts(suite, vm, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().Profile = "rack-a"

		return nil
	})
	assertRendered("pc", libvirtxml.DomainCPU{Mode: "host-passthrough"})

	rendered, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), vm.Metadata().ID())
	suite.Require().NoError(err)
	profile, err = safe.StateGetByID[*hypervisor.VirtualMachineProfileSpec](suite.Ctx(), suite.State(), "rack-a")
	suite.Require().NoError(err)
	suite.Destroy(profile)
	ctest.AssertResource(suite, vm.Metadata().ID(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", res.TypedSpec().PowerState)
		asrt.Equal(rendered.TypedSpec().DomainXML, res.TypedSpec().DomainXML)
	})

	profile.Metadata().SetVersion(resource.VersionUndefined)
	suite.Create(profile)
	assertRendered("pc", libvirtxml.DomainCPU{Mode: "host-passthrough"})
}

func (suite *VirtualMachineSpecSuite) TestProfileRendersGuestHardwareAndMissingProfile() {
	if runtime.GOARCH != "amd64" {
		suite.T().Skip("VM hardware profiles are only supported on amd64")
	}

	name := "profile-guest"
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 2},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
		PowerState: "stopped",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Profile:    "rack-a",
	}
	suite.Create(spec)
	suite.Require().Eventually(func() bool {
		return suite.logs.Filter(func(entry observer.LoggedEntry) bool {
			message, ok := entry.ContextMap()["error"].(string)

			return ok && strings.Contains(message, `virtual machine "profile-guest": profile "rack-a" not found`)
		}).Len() > 0
	}, 5*time.Second, 10*time.Millisecond)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, name)

	profile := hypervisor.NewVirtualMachineProfileSpec(hypervisor.NamespaceName, "rack-a")
	*profile.TypedSpec() = hypervisor.VirtualMachineProfileSpecSpec{
		Machine: "pc-q35-9.2",
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode:     hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			Model:    "x86-64-v3",
			Fallback: "forbid",
			Features: []hypervisor.VirtualMachineProfileCPUFeatureSpec{
				{
					Name:   "pku",
					Policy: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyDisable,
				},
			},
		},
	}
	suite.Create(profile)
	ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		var domain libvirtxml.Domain
		if !asrt.NoError(domain.Unmarshal(res.TypedSpec().DomainXML)) || !asrt.NotNil(domain.CPU) || !asrt.NotNil(domain.CPU.Model) {
			return
		}

		asrt.Equal("pc-q35-9.2", domain.OS.Type.Machine)
		asrt.Equal("custom", domain.CPU.Mode)
		asrt.Equal("x86-64-v3", domain.CPU.Model.Value)
		asrt.Equal("forbid", domain.CPU.Model.Fallback)

		if asrt.Len(domain.CPU.Features, 1) {
			asrt.Equal("pku", domain.CPU.Features[0].Name)
			asrt.Equal("disable", domain.CPU.Features[0].Policy)
		}
	})
}

// Video model + VRAM flow through to <video><model type="…" vram="…">; empty model falls
// back to "vga" and zero vramMiB omits the attribute.
func (suite *VirtualMachineSpecSuite) TestVideoDevice() {
	for _, tc := range []struct {
		name    string
		video   hypervisor.VirtualMachineVideoSpec
		model   string
		vramKiB uint
		noVNC   bool
	}{
		{name: "default", model: "vga"},
		{name: "default-vram", video: hypervisor.VirtualMachineVideoSpec{VRAMMiB: 32}, model: "vga", vramKiB: 32 * 1024},
		{name: "vga-512", video: hypervisor.VirtualMachineVideoSpec{Model: "vga", VRAMMiB: 512}, model: "vga", vramKiB: 512 * 1024},
		{name: "bochs-32", video: hypervisor.VirtualMachineVideoSpec{Model: "bochs", VRAMMiB: 32}, model: "bochs", vramKiB: 32 * 1024},
		{name: "virtio-no-vram", video: hypervisor.VirtualMachineVideoSpec{Model: "virtio"}, model: "virtio"},
		{name: "cirrus-no-vram", video: hypervisor.VirtualMachineVideoSpec{Model: "cirrus"}, model: "cirrus"},
		{name: "no-vnc", video: hypervisor.VirtualMachineVideoSpec{Model: "bochs", VRAMMiB: 32}, noVNC: true},
	} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "video-"+tc.name)
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
			PowerState: "stopped",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Console:    hypervisor.VirtualMachineConsoleSpec{VNC: !tc.noVNC},
			Video:      tc.video,
		}
		suite.Create(spec)

		ctest.AssertResource(suite, "video-"+tc.name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			var domain libvirtxml.Domain
			if !asrt.NoError(domain.Unmarshal(res.TypedSpec().DomainXML)) {
				return
			}

			asrt.NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))

			if tc.noVNC || runtime.GOARCH != "amd64" {
				asrt.Empty(domain.Devices.Videos)

				return
			}

			if !asrt.Len(domain.Devices.Videos, 1) {
				return
			}

			asrt.Equal(tc.model, domain.Devices.Videos[0].Model.Type)
			asrt.Equal(tc.vramKiB, domain.Devices.Videos[0].Model.VRam)
		})
	}
}

// Both production controllers remain registered in these tests. No MachineConfig
// is needed to create, update, validate, or remove externally authored specs.
func (suite *VirtualMachineSpecSuite) TestExternalSpecLifecycle() {
	suite.externalSpecLifecycle("external")
}

func (suite *VirtualMachineSpecSuite) TestUnownedSpecLifecycle() {
	suite.externalSpecLifecycle("")
}

func (suite *VirtualMachineSpecSuite) externalSpecLifecycle(owner string) {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "balloon")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU: hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory: hypervisor.VirtualMachineMemorySpec{
			Size:       4 << 30,
			Ballooning: hypervisor.VirtualMachineMemoryBallooningSpec{Enabled: true},
		},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), spec, state.WithCreateOwner(owner)))
	suite.assertDomain("balloon", "balloon-enabled")
	ctest.AssertNoResource[*config.MachineConfig](suite, config.ActiveID)

	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), "balloon")
	suite.Require().NoError(err)

	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 7},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Require().NoError(suite.State().Update(suite.Ctx(), spec, state.WithUpdateOwner(owner)))
	suite.assertDomain("balloon", "balloon-omitted")

	// A config reconcile and subsequent cleanup must leave external input alone.
	cfg, err := container.New(newVirtualMachine("configured"))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, "configured", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	suite.Destroy(config.NewMachineConfig(cfg))
	ctest.AssertNoResource[*hypervisor.VirtualMachineSpec](suite, "configured")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "configured")
	ctest.AssertResource(suite, "balloon", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal(owner, res.Metadata().Owner())
		asrt.Equal(spec.TypedSpec(), res.TypedSpec())
	})
	suite.assertDomain("balloon", "balloon-omitted")

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), spec.Metadata(), state.WithDestroyOwner(owner)))
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "balloon")
}

func (suite *VirtualMachineSpecSuite) TestBIOSDomainUsesDefaultFirmware() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "bios-default")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 128 << 20},
		PowerState: "stopped",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "bios"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(spec)
	ctest.AssertResource(suite, spec.Metadata().ID(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, "<os>")
		asrt.NotContains(res.TypedSpec().DomainXML, `firmware="bios"`)
		asrt.Equal("stopped", res.TypedSpec().PowerState)
		// Regression guard: ACPI must stay enabled for BIOS firmware too.
		asrt.Contains(res.TypedSpec().DomainXML, "<acpi></acpi>")
	})
}

// Every domain gets ACPI, not only the UEFI ones which cannot boot without it: without it a
// guest has no power button, so it could only ever be stopped by being destroyed.
func (suite *VirtualMachineSpecSuite) TestEveryDomainEnablesACPI() {
	for _, firmware := range []string{"bios", "uefi"} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, firmware+"-acpi")
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
			PowerState: "stopped",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: firmware},
		}
		suite.Create(spec)
		ctest.AssertResource(suite, spec.Metadata().ID(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Contains(res.TypedSpec().DomainXML, "<acpi></acpi>")
		})
	}
}

// The stop mode asked for through the API reaches the controller which takes the domain down, and
// a virtual machine nothing asked anything of is stopped the way every stop used to be.
func (suite *VirtualMachineSpecSuite) TestStopModeReachesTheDomainSpec() {
	for _, stopMode := range []string{"", hypervisorhelpers.StopModeGraceful.String(), hypervisorhelpers.StopModeForced.String()} {
		name := "stop-mode-" + stopMode

		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)

		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
			PowerState: "stopped",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		}

		request := ""

		if stopMode != "" {
			request = stopMode + "/" + name

			override := hypervisor.NewVirtualMachinePowerOverride(hypervisor.NamespaceName, name)
			override.TypedSpec().PowerState = "stopped"
			override.TypedSpec().StopRequest = request
			suite.Create(override)
		}

		suite.Create(spec)
		ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Equal(stopMode, res.TypedSpec().StopMode)
			asrt.Equal(request, res.TypedSpec().StopRequest)
			asrt.Equal("stopped", res.TypedSpec().PowerState)
		})

		if stopMode != "" {
			suite.Destroy(hypervisor.NewVirtualMachinePowerOverride(hypervisor.NamespaceName, name))
			ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
				asrt.Equal("stopped", res.TypedSpec().PowerState)
				asrt.Empty(res.TypedSpec().StopRequest)
				asrt.Empty(res.TypedSpec().StopMode)
			})
		}
	}
}

// A stopped virtual machine whose stop request outlives its stop is still rendered: editing it
// while it is stopped reaches its domain spec.
func (suite *VirtualMachineSpecSuite) TestStoppedVirtualMachineEditReachesTheDomainSpec() {
	name := "stopped-edit"
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
		PowerState: "stopped",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
	}

	override := hypervisor.NewVirtualMachinePowerOverride(hypervisor.NamespaceName, name)
	override.TypedSpec().PowerState = "stopped"
	override.TypedSpec().StopRequest = hypervisorhelpers.StopModeGraceful.String() + "/request-a"
	suite.Create(override)
	suite.Create(spec)
	ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, "<vcpu>1</vcpu>")
	})

	ctest.UpdateWithConflicts(suite, spec, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().CPU.Count = 2

		return nil
	})
	ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, "<vcpu>2</vcpu>")
		asrt.Equal(hypervisorhelpers.StopModeGraceful.String(), res.TypedSpec().StopMode)
	})
}

func (suite *VirtualMachineSpecSuite) TestDomainSpecCleanupWaitsForFinalizer() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "held-domain")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
		PowerState: "stopped",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(spec)

	domain := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, spec.Metadata().ID())
	ctest.AssertResource(suite, domain.Metadata().ID(), func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	suite.AddFinalizer(domain.Metadata(), "test.libvirt-cleanup")
	suite.Destroy(spec)

	ctest.AssertResource(suite, domain.Metadata().ID(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
		asrt.True(res.Metadata().Finalizers().Has("test.libvirt-cleanup"))
	})
	suite.RemoveFinalizer(domain.Metadata(), "test.libvirt-cleanup")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, domain.Metadata().ID())
}

func (suite *VirtualMachineSpecSuite) TestExternalSpecRejectsConfigCollision() {
	for _, owner := range []string{"", "external"} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		}
		suite.Require().NoError(suite.State().Create(suite.Ctx(), spec, state.WithCreateOwner(owner)))
		suite.assertDomain("guest-one", "default")
		suite.logs.TakeAll()

		doc := newVirtualMachine("guest-one")
		doc.CPUConfig.CPUCount = 7
		cfg, err := container.New(doc)
		suite.Require().NoError(err)
		suite.Create(config.NewMachineConfig(cfg))
		suite.Require().Eventually(func() bool {
			return suite.logs.Filter(func(entry observer.LoggedEntry) bool {
				message, ok := entry.ContextMap()["error"].(string)

				return ok && strings.Contains(message, `failed to write virtual machine spec "guest-one"`)
			}).Len() > 0
		}, 3*time.Second, time.Millisecond)
		ctest.AssertResource(suite, "guest-one", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
			asrt.Equal(owner, res.Metadata().Owner())
			asrt.Equal(spec.TypedSpec(), res.TypedSpec())
		})
		suite.assertDomain("guest-one", "default")

		// Clear the conflict and force a successful config pass before checking cleanup.
		suite.replaceConfig(newVirtualMachine("barrier"))
		ctest.AssertResource(suite, "barrier", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
		suite.Destroy(config.NewMachineConfig(cfg))
		ctest.AssertNoResource[*hypervisor.VirtualMachineSpec](suite, "barrier")
		suite.assertDomain("guest-one", "default")
		suite.Require().NoError(suite.State().Destroy(suite.Ctx(), spec.Metadata(), state.WithDestroyOwner(owner)))
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "guest-one")
	}
}

func (suite *VirtualMachineSpecSuite) TestPersistentCollisionDoesNotBlockProjectionOrCleanup() {
	cfg, err := container.New(newVirtualMachine("removed"), newVirtualMachine("retained"))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, "removed", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	ctest.AssertResource(suite, "retained", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})

	external := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
	*external.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), external, state.WithCreateOwner("external")))
	suite.assertDomain("guest-one", "default")

	// A phase conflict exercises failed modification tracking for an owned spec.
	retained, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), "retained")
	suite.Require().NoError(err)
	_, err = suite.State().Teardown(suite.Ctx(), retained.Metadata(), state.WithTeardownOwner(retained.Metadata().Owner()))
	suite.Require().NoError(err)

	collision := newVirtualMachine("guest-one")
	collision.CPUConfig.CPUCount = 7
	retainedDoc := newVirtualMachine("retained")
	retainedDoc.CPUConfig.CPUCount = 7
	suite.replaceConfig(collision, retainedDoc, newVirtualMachine("balloon"))
	suite.assertDomain("balloon", "balloon-disabled")
	ctest.AssertNoResource[*hypervisor.VirtualMachineSpec](suite, "removed")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "removed")
	ctest.AssertResource(suite, "retained", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal(retained.Metadata().Owner(), res.Metadata().Owner())
		asrt.Equal(retained.TypedSpec(), res.TypedSpec())
	})
	ctest.AssertResource(suite, "guest-one", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal("external", res.Metadata().Owner())
		asrt.Equal(external.TypedSpec(), res.TypedSpec())
	})
	suite.assertDomain("guest-one", "default")
}

func (suite *VirtualMachineSpecSuite) TestEqualConfigCollisionPreservesExternalOwnership() {
	for _, owner := range []string{"", "external"} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			Profile:    "host",
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		}
		suite.Require().NoError(suite.State().Create(suite.Ctx(), spec, state.WithCreateOwner(owner)))
		suite.assertDomain("guest-one", "default")

		cfg, err := container.New(newVirtualMachine("guest-one"), newVirtualMachine("barrier"))
		suite.Require().NoError(err)
		suite.logs.TakeAll()
		suite.Create(config.NewMachineConfig(cfg))
		ctest.AssertResource(suite, "barrier", func(_ *hypervisor.VirtualMachineSpec, _ *assert.Assertions) {})
		suite.Destroy(config.NewMachineConfig(cfg))
		ctest.AssertNoResource[*hypervisor.VirtualMachineSpec](suite, "barrier")
		ctest.AssertResource(suite, "guest-one", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
			asrt.Equal(owner, res.Metadata().Owner())
			asrt.Equal(spec.TypedSpec(), res.TypedSpec())
			asrt.Equal(spec.Metadata().Version(), res.Metadata().Version())
		})
		suite.Empty(suite.logs.FilterLevelExact(zap.ErrorLevel).All())
		suite.assertDomain("guest-one", "default")
		suite.Require().NoError(suite.State().Destroy(suite.Ctx(), spec.Metadata(), state.WithDestroyOwner(owner)))
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "guest-one")
	}
}

func (suite *VirtualMachineSpecSuite) TestInjectedInvalidNames() {
	for _, name := range []string{"", "bad\nname", "bad\x00name", "bad\x01name", "bad\ufffename", "bad\uffffname", "bad\xffname", "bad\xed\xa0\x80name"} {
		suite.Run(fmt.Sprintf("%q", name), func() {
			suite.logs.TakeAll()

			spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
			*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
				CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
				Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
				PowerState: "running",
				Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
				Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
			}

			suite.Create(spec)
			defer suite.Destroy(spec)

			suite.assertConversionError(name, "")
			ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, name)
		})
	}
}

func (suite *VirtualMachineSpecSuite) TestInjectedEscapedName() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest/&<\u96ea>\t\r🙂")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(spec)
	suite.assertDomain(spec.Metadata().ID(), "escaped-name")
}

// Resource IDs are not machine-configuration names. Each escaped ID must reach
// the storage request boundary without being treated as a path or truncated.
func (suite *VirtualMachineDiskSuite) TestInjectedEscapedBlankVolumeNames() {
	ids := []string{
		"guest/&<\u96ea>	\r🙂",
		"guest/&<\u96ea>	\r🙂-other",
		"../guest",
		"guest__data",
		strings.Repeat("long", 1024),
		strings.Repeat("long", 1024) + "different",
		"vm-a",
		"vm",
	}
	seen := map[string]struct{}{}

	for _, id := range ids {
		for _, diskName := range []string{"data", "a-data"} {
			disk := blankDiskSpec(diskName, "pool1", "raw", 1<<20)
			vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, id)

			vm.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{disk}
			if diskName == "data" {
				suite.Create(vm)
			} else {
				ctest.UpdateWithConflicts(suite, vm, func(current *hypervisor.VirtualMachineSpec) error {
					current.TypedSpec().Disks = append(current.TypedSpec().Disks, disk)

					return nil
				})
			}

			var volume string

			ctest.AssertResource(suite, hypervisor.VirtualMachineDiskStatusID(id, disk), func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
				asrt.NotEmpty(status.TypedSpec().Volume)
				asrt.NotContains(status.TypedSpec().Error, "unsupported")
				volume = status.TypedSpec().Volume
			})
			suite.Require().NotEmpty(volume)
			suite.Require().LessOrEqual(len(volume), 255)
			suite.Require().NotContains(volume, "/")
			suite.Require().Equal(filepath.Base(volume), volume)
			_, duplicate := seen[volume]
			suite.Require().False(duplicate, "distinct ID/disk pairs must not share a volume")

			seen[volume] = struct{}{}
			suite.Equal(volume, hypervisorctrl.BlankVolumeNameForTest(id, disk), "mapping must be stable")

			if id == "vm" || id == "vm-a" {
				suite.Equal(id+"__"+diskName+".raw", volume, "preserve existing filenames")
			}

			ctest.AssertResource(suite, storage.StoragePoolVolumeID("pool1", volume), func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
				asrt.Equal(volume, request.TypedSpec().Name)
				asrt.Equal(uint64(1<<20), request.TypedSpec().Capacity)
			})
		}
	}
}

func (suite *VirtualMachineSpecSuite) TestInjectedInvalidSpecs() {
	for _, invalid := range []hypervisor.VirtualMachineSpecSpec{
		// Missing power and firmware are independently invalid even with valid CPU/memory.
		{
			CPU:      hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:   hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Firmware: hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:    hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		},
		{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024},
			PowerState: "running",
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 0},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1024},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 65536},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1024},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 4294967295},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1024},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 0},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1023},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1025},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1 << 63},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1<<64 - 1},
		},
		// Interface names become libvirt user aliases and must be unique; links are required.
		{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{{Name: "", Link: "eth0"}},
		},
		{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{{Name: "net.0", Link: "eth0"}},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{
				{Name: "net0", Link: "eth0"},
				{Name: "net0", Link: "eth1"},
			},
		},
		{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{{Name: "net0"}},
		},
	} {
		suite.logs.TakeAll()

		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "invalid")

		*spec.TypedSpec() = invalid
		if invalid.PowerState == "" && invalid.Firmware.Type == "" {
			// CPU/memory cases must reach their own validation, not fail on missing intent.
			spec.TypedSpec().PowerState = "running"
			spec.TypedSpec().Firmware.Type = "uefi"
		}

		suite.Create(spec)

		reason := ""
		if invalid.Firmware.Type != "" && invalid.PowerState == "" {
			reason = `unsupported power state ""`
		} else if invalid.PowerState != "" && invalid.Firmware.Type == "" {
			reason = `unsupported firmware type ""`
		}

		suite.assertConversionError("invalid", reason)
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "invalid")
		suite.Destroy(spec)

		// A successful render resets restart backoff between invalid cases.
		barrier := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "balloon")
		*barrier.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		}
		suite.Create(barrier)
		suite.assertDomain("balloon", "balloon-disabled")
		suite.Destroy(barrier)
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "balloon")
	}
}

func (suite *VirtualMachineSpecSuite) TestInvalidSpecDoesNotBlockOtherDomains() {
	stale := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "balloon")
	*stale.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(stale)
	suite.assertDomain("balloon", "balloon-disabled")

	invalid := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "aaa-invalid")
	suite.Create(invalid)
	suite.assertConversionError("aaa-invalid", "")
	suite.Destroy(stale)

	healthy := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
	*healthy.TypedSpec() = *stale.TypedSpec()
	suite.Create(healthy)
	suite.assertDomain("guest-one", "default")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "balloon")
}

func (suite *VirtualMachineSpecSuite) TestInjectedInvalidUpdateRecovers() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "balloon")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(spec)
	suite.assertDomain("balloon", "balloon-disabled")
	suite.logs.TakeAll()

	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), "balloon")
	suite.Require().NoError(err)

	spec.TypedSpec().Memory.Size++
	suite.Update(spec)
	suite.assertConversionError("balloon", "memory size must be a multiple of 1024 bytes")
	suite.assertDomain("balloon", "balloon-disabled")

	spec, err = safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), "balloon")
	suite.Require().NoError(err)

	spec.TypedSpec().Memory.Size--
	spec.TypedSpec().Memory.Ballooning.Enabled = true
	suite.Update(spec)
	suite.assertDomain("balloon", "balloon-enabled")
	suite.Destroy(spec)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "balloon")
}

// Register an alternate COSI producer, rather than merely asserting an output
// declaration. The runtime must allow it alongside the machine-config projector.
func (suite *VirtualMachineSpecSuite) TestAllowsSharedSpecProducer() {
	suite.Require().NoError(suite.Runtime().RegisterController(&externalSpecProducer{}))
}

type externalSpecProducer struct {
	hypervisorctrl.VirtualMachineSpecController
}

func (*externalSpecProducer) Name() string {
	return "external.VirtualMachineSpecController"
}

func (*externalSpecProducer) Inputs() []controller.Input {
	return nil
}

func (*externalSpecProducer) Run(ctx context.Context, _ controller.Runtime, _ *zap.Logger) error {
	<-ctx.Done()

	return nil
}

// contentLibraryPlaceholder stands in for the library's mount point, which is a temporary
// directory and therefore differs between runs, so that the fixture can stay exact.
const contentLibraryPlaceholder = "/content-library"

// publishCDROMCopy completes the disk controller's pool copy request without running storage.
func (suite *VirtualMachineSpecSuite) publishCDROMCopy(vmName, diskName, poolPath string) string {
	suite.T().Helper()

	var volumeName string

	suite.Require().Eventually(func() bool {
		vm, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), vmName)
		if err != nil || len(vm.TypedSpec().Disks) == 0 {
			return false
		}

		for _, disk := range vm.TypedSpec().Disks {
			if disk.Name != diskName {
				continue
			}

			status, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](suite.Ctx(), suite.State(), hypervisor.VirtualMachineDiskStatusID(vmName, disk))
			if err == nil && status.TypedSpec().Volume != "" {
				volumeName = status.TypedSpec().Volume

				return true
			}
		}

		return false
	}, 5*time.Second, 10*time.Millisecond)

	path := filepath.Join(poolPath, volumeName)
	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, storage.StoragePoolVolumeID("pool1", volumeName))
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool: "pool1", Name: volumeName, Path: path, Format: "raw", Phase: storage.StoragePoolVolumePhaseReady,
	}
	suite.Create(volume)

	return path
}

func (suite *VirtualMachineSpecSuite) TestRendersCDROMFromContentLibrary() {
	path := suite.T().TempDir()
	writeLibraryImage(suite.T(), path, "talos.iso", "iso")

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	*library.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		VolumeID: "u-images",
		Path:     path,
		Phase:    hypervisor.ContentLibraryPhaseReady,
	}
	suite.Create(library)

	doc := newVirtualMachine("booted")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:      "install",
			DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskPool:      "pool1",
			DiskFormat:    hypervisorhelpers.VirtualMachineDiskFormatRaw,
			DiskBootOrder: 1,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: "images",
					ImageFile:    "talos.iso",
				},
			},
		},
	}

	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	copyPath := suite.publishCDROMCopy(doc.Name(), "install", suite.T().TempDir())

	want, err := os.ReadFile(filepath.Join("testdata", "virtualmachinespec", "cdrom.xml"))
	suite.Require().NoError(err)

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal(string(want), strings.ReplaceAll(res.TypedSpec().DomainXML+"\n", copyPath, "/storage-pool/install.iso"))
	})

	res, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), doc.Name())
	suite.Require().NoError(err)
	suite.Require().NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))
}

// Two cdroms on the same driver must not both claim sda.
func (suite *VirtualMachineSpecSuite) TestAllocatesDistinctTargetDevices() {
	path := suite.T().TempDir()
	writeLibraryImage(suite.T(), path, "talos.iso", "iso")

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "two-images")
	*library.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		VolumeID: "u-two-images",
		Path:     path,
		Phase:    hypervisor.ContentLibraryPhaseReady,
	}
	suite.Create(library)

	cdrom := func(name string, bus hypervisorhelpers.VirtualMachineDiskBus) hypervisorcfg.VirtualMachineDisk {
		return hypervisorcfg.VirtualMachineDisk{
			DiskName:   name,
			DiskType:   hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskPool:   "pool1",
			DiskFormat: hypervisorhelpers.VirtualMachineDiskFormatRaw,
			DiskBus:    bus,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: "two-images",
					ImageFile:    "talos.iso",
				},
			},
		}
	}

	doc := newVirtualMachine("two-cdroms")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		cdrom("install", hypervisorhelpers.VirtualMachineDiskBusSATA),
		cdrom("rescue", hypervisorhelpers.VirtualMachineDiskBusSCSI),
	}

	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.publishCDROMCopy(doc.Name(), "install", suite.T().TempDir())
	suite.publishCDROMCopy(doc.Name(), "rescue", suite.T().TempDir())

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, `<target dev="sda" bus="sata"></target>`)
		asrt.Contains(res.TypedSpec().DomainXML, `<target dev="sdb" bus="scsi"></target>`)
	})
}

// libvirt has no nvme disk bus, so the domain is refused rather than rendered unusably.
func (suite *VirtualMachineSpecSuite) TestRejectsNVMeBus() {
	path := suite.T().TempDir()
	writeLibraryImage(suite.T(), path, "talos.iso", "iso")

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "nvme-images")
	*library.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		VolumeID: "u-nvme-images",
		Path:     path,
		Phase:    hypervisor.ContentLibraryPhaseReady,
	}
	suite.Create(library)

	doc := newVirtualMachine("nvme-cdrom")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:   "install",
			DiskType:   hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskPool:   "pool1",
			DiskFormat: hypervisorhelpers.VirtualMachineDiskFormatRaw,
			DiskBus:    hypervisorhelpers.VirtualMachineDiskBusNVMe,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: "nvme-images",
					ImageFile:    "talos.iso",
				},
			},
		},
	}

	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	suite.publishCDROMCopy(doc.Name(), "install", suite.T().TempDir())
	suite.assertConversionError(doc.Name(), `disk "install": unsupported bus "nvme"`)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
}

// VirtualMachineStaleDiskSuite runs the domain renderer without VirtualMachineDiskController, so
// that the disk statuses it reads are the ones the test writes.
type VirtualMachineStaleDiskSuite struct {
	ctest.DefaultSuite
}

func TestVirtualMachineStaleDiskSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VirtualMachineStaleDiskSuite{
		Timeout: 15 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineProfileSpecController{}))
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
		},
	})
}

func (suite *VirtualMachineStaleDiskSuite) TestDiskPhasesGateDomainRendering() {
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	*vm.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU: hypervisor.VirtualMachineCPUSpec{
			Count: 1,
		},
		Memory: hypervisor.VirtualMachineMemorySpec{
			Size: 512 << 20,
		},
		PowerState: "running",
		Firmware: hypervisor.VirtualMachineFirmwareSpec{
			Type: "bios",
		},
		Disks: []hypervisor.VirtualMachineDiskSpec{disk},
	}
	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, disk))
	*status.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: vmName,
		Name:           disk.Name,
		SourcePath:     filepath.Join(contentLibraryPlaceholder, "talos.iso"),
		Format:         "raw",
		ReadOnly:       true,
	}
	suite.Create(status)
	suite.Create(vm)

	ctx, st := suite.Ctx(), suite.State()

	for _, phase := range []hypervisor.VirtualMachineDiskPhase{
		hypervisor.VirtualMachineDiskPhaseUnknown,
		hypervisor.VirtualMachineDiskPhaseNotReady,
		hypervisor.VirtualMachineDiskPhaseObservationUnavailable,
	} {
		ctest.UpdateWithConflicts(suite, status, func(current *hypervisor.VirtualMachineDiskStatus) error {
			current.TypedSpec().Phase = phase

			return nil
		})
		suite.Require().Never(func() bool {
			_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](ctx, st, vmName)

			return !state.IsNotFoundError(err)
		}, 100*time.Millisecond, 10*time.Millisecond)
	}

	ctest.UpdateWithConflicts(suite, status, func(current *hypervisor.VirtualMachineDiskStatus) error {
		current.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseReady

		return nil
	})
	ctest.AssertResource(suite, vmName, func(domain *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", domain.TypedSpec().PowerState)
	})

	ctest.UpdateWithConflicts(suite, status, func(current *hypervisor.VirtualMachineDiskStatus) error {
		current.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseUnknown

		return nil
	})
	ctest.AssertResource(suite, vmName, func(domain *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", domain.TypedSpec().PowerState)
	})
}

// Disk observation can lag behind the pool's recovered Ready status.
func (suite *VirtualMachineStaleDiskSuite) TestRecoveredPoolBeforeDiskObservation() {
	disk := hypervisor.VirtualMachineDiskSpec{
		Name:   "system",
		Pool:   "vms",
		Type:   "disk",
		Bus:    "virtio",
		Format: "raw",
		Size:   1 << 20,
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			Blank: true,
		},
	}
	otherDisk := disk
	otherDisk.Name = "data"

	poolSpec := storage.NewStoragePoolSpec(storage.NamespaceName, "vms")
	poolSpec.TypedSpec().VolumeID = "u-vms"
	suite.Create(poolSpec)

	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "vms")
	pool.TypedSpec().VolumeID = "u-vms"
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	suite.Create(pool)

	mount := block.NewVolumeMountStatus(block.NamespaceName, pool.TypedSpec().VolumeID)
	mount.TypedSpec().Target = suite.T().TempDir()
	suite.Create(mount)

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, disk))
	*status.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: vmName, Name: disk.Name, Blank: true, Pool: disk.Pool, Volume: "vm__system.raw",
		SourcePath: "/tmp/vm__system.raw",
		Format:     "raw",
		Size:       disk.Size,
		Phase:      hypervisor.VirtualMachineDiskPhaseReady,
	}
	suite.Create(status)

	otherStatus := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, otherDisk))
	*otherStatus.TypedSpec() = *status.TypedSpec()
	otherStatus.TypedSpec().Name = otherDisk.Name
	otherStatus.TypedSpec().Volume = "vm__data.raw"
	otherStatus.TypedSpec().SourcePath = "/tmp/vm__data.raw"
	suite.Create(otherStatus)

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	*vm.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
		PowerState: "running", Firmware: hypervisor.VirtualMachineFirmwareSpec{Type: "bios"},
		Disks: []hypervisor.VirtualMachineDiskSpec{disk, otherDisk},
	}
	suite.Create(vm)

	var original string

	suite.Require().Eventually(func() bool {
		domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), vmName)
		if err != nil || domain.TypedSpec().DomainXML == "" || domain.TypedSpec().PowerState != "running" {
			return false
		}

		original = domain.TypedSpec().DomainXML

		return true
	}, 5*time.Second, 10*time.Millisecond)

	ctest.UpdateWithConflicts(suite, status, func(current *hypervisor.VirtualMachineDiskStatus) error {
		current.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseObservationUnavailable
		current.TypedSpec().Error = "storage observation unavailable"

		return nil
	})
	// Pool is already Ready: current pool state alone cannot explain the stale disk.
	ctx := suite.Ctx()
	st := suite.State()
	suite.Require().Never(func() bool {
		domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](ctx, st, vmName)

		return err != nil || domain.TypedSpec().PowerState != "running" || domain.TypedSpec().DomainXML != original
	}, 200*time.Millisecond, 10*time.Millisecond)

	// Another attached disk disappearing during the outage must stop the guest,
	// even though the renderer reports the first disk's observation error.
	suite.Destroy(otherStatus)
	ctest.AssertResource(suite, vmName, func(domain *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", domain.TypedSpec().PowerState)
	})

	// Restore the missing disk and a fully observed definition before independently
	// exercising the owner's retarget signal.
	otherStatus.Metadata().SetVersion(resource.VersionUndefined)
	suite.Create(otherStatus)
	ctest.UpdateWithConflicts(suite, status, func(current *hypervisor.VirtualMachineDiskStatus) error {
		current.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseReady

		return nil
	})
	ctest.AssertResource(suite, vmName, func(domain *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", domain.TypedSpec().PowerState)
	})
	ctest.UpdateWithConflicts(suite, status, func(current *hypervisor.VirtualMachineDiskStatus) error {
		current.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseObservationUnavailable

		return nil
	})

	// Storage owns retarget/mount lifecycle and reports intentional withdrawal as
	// NotReady; the renderer reads status, never reconstructs the pool specification.
	ctest.UpdateWithConflicts(suite, pool, func(current *storage.StoragePoolStatus) error {
		current.TypedSpec().Phase = storage.StoragePoolPhaseNotReady
		current.TypedSpec().Error = "backing retarget held"

		return nil
	})
	ctest.AssertResource(suite, vmName, func(domain *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", domain.TypedSpec().PowerState)
	})
}

// A disk status is keyed by what the disk is provisioned from, so a status left over from another
// image is a different resource and is simply not found. Rendering from it would attach the
// previous image, and a rendered domain is started.
func (suite *VirtualMachineStaleDiskSuite) TestWaitsOutADiskStatusForAnotherImage() {
	wanted := cdromDiskSpec("install", libraryName, "new.iso", "")

	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		Disks:      []hypervisor.VirtualMachineDiskSpec{wanted},
	}
	suite.Create(spec)

	stale := cdromDiskSpec("install", libraryName, "old.iso", "")

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, stale))
	*status.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: vmName,
		Name:           "install",
		SourcePath:     filepath.Join(contentLibraryPlaceholder, "old.iso"),
		Format:         "raw",
		ReadOnly:       true,
		Phase:          hypervisor.VirtualMachineDiskPhaseReady,
		Image:          hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "old.iso"},
	}
	suite.Create(status)

	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, vmName)

	current := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, wanted))
	*current.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: vmName,
		Name:           "install",
		SourcePath:     filepath.Join(contentLibraryPlaceholder, "new.iso"),
		Format:         "raw",
		ReadOnly:       true,
		Phase:          hypervisor.VirtualMachineDiskPhaseReady,
		Image:          hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "new.iso"},
	}
	suite.Create(current)

	ctest.AssertResource(suite, vmName, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, filepath.Join(contentLibraryPlaceholder, "new.iso"))
		asrt.NotContains(res.TypedSpec().DomainXML, "old.iso")
		// The definition carries what it was rendered from, so the controller which starts it can
		// hold exactly those disks.
		asrt.Equal([]string{current.Metadata().ID()}, res.TypedSpec().Disks)
	})
}

// A status held by a domain goes on existing while it tears down, but names an image the
// configuration has moved off. Rendering from it would keep a guest on a medium whose digest is
// never verified again, so the domain is stopped instead and the hold comes back with it.
func (suite *VirtualMachineStaleDiskSuite) TestStopsOnADiskStatusWhichIsTearingDown() {
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")

	// A graceful stop was asked for, which the withdrawal below is entitled to ignore.
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		Disks:      []hypervisor.VirtualMachineDiskSpec{disk},
	}
	suite.Create(spec)

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, disk))
	*status.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: vmName,
		Name:           "install",
		SourcePath:     filepath.Join(contentLibraryPlaceholder, "talos.iso"),
		Format:         "raw",
		ReadOnly:       true,
		Phase:          hypervisor.VirtualMachineDiskPhaseReady,
		Image:          hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "talos.iso"},
	}
	suite.Create(status)

	ctest.AssertResource(suite, vmName, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", res.TypedSpec().PowerState)
		asrt.Equal([]string{status.Metadata().ID()}, res.TypedSpec().Disks)
	})

	override := hypervisor.NewVirtualMachinePowerOverride(hypervisor.NamespaceName, vmName)
	override.TypedSpec().PowerState = "stopped"
	override.TypedSpec().StopRequest = hypervisorhelpers.StopModeGraceful.String() + "/" + vmName
	suite.Create(override)
	ctest.AssertResource(suite, vmName, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", res.TypedSpec().PowerState)
		asrt.Equal(hypervisorhelpers.StopModeGraceful.String(), res.TypedSpec().StopMode)
		asrt.Equal(override.TypedSpec().StopRequest, res.TypedSpec().StopRequest)
	})

	suite.AddFinalizer(status.Metadata(), "hypervisor.VirtualMachineController")

	ready, err := suite.State().Teardown(suite.Ctx(), status.Metadata())
	suite.Require().NoError(err)
	suite.Require().False(ready, "the hold must prevent immediate deletion")

	ctest.AssertResource(suite, vmName, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", res.TypedSpec().PowerState)
		// A stop the controller demands to clear an unrenderable definition off the host is not
		// one the guest may refuse, whatever was asked for.
		asrt.Equal(hypervisorhelpers.StopModeForced.String(), res.TypedSpec().StopMode)
	})
}

type VirtualMachineCloudInitDomainSuite struct {
	ctest.DefaultSuite
}

func TestVirtualMachineCloudInitDomainSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VirtualMachineCloudInitDomainSuite{
		ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.CloudInitSpecController{}))

				s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineProfileSpecController{}))
				s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
			},
		},
	})
}

func (s *VirtualMachineCloudInitDomainSuite) TestSeedIsAttachedOnlyWhenReady() {
	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest")
	vm.TypedSpec().CPU.Count = 1
	vm.TypedSpec().Memory.Size = 512 << 20
	vm.TypedSpec().PowerState = "running"
	vm.TypedSpec().Firmware.Type = "bios"
	vm.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{Pool: "pool-a", MetaData: "instance-id: guest\n", UserData: "secret"}
	s.Create(vm)

	// No pool-backed seed: rendering must not start an unseeded guest.
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](s, "guest")

	dir := s.T().TempDir()
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool-a")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	pool.TypedSpec().TargetPath = dir
	pool.TypedSpec().VolumeID = "volume-a"
	s.Create(pool)
	// Pool readiness alone does not grant a seed attachment.
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](s, "guest")

	seed := hypervisor.CloudInitSpecSpec{Pool: "pool-a", MetaData: "instance-id: guest\n", UserData: "secret"}
	id := hypervisor.CloudInitStatusID("guest", seed)

	ctest.AssertResource(s, "guest", func(projected *hypervisor.CloudInitSpec, a *assert.Assertions) {
		a.Equal(seed, *projected.TypedSpec())
	})
	projected, err := safe.StateGetByID[*hypervisor.CloudInitSpec](s.Ctx(), s.State(), "guest")
	s.Require().NoError(err)

	name := "cloud-init-guest.iso"
	path := filepath.Join(dir, name)
	s.Require().NoError(os.WriteFile(path, []byte("seed-content"), 0o600))

	asset := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id)
	*asset.TypedSpec() = hypervisor.CloudInitStatusSpec{
		VirtualMachine: "guest", Pool: "pool-a", Name: name,
		Path: dir, VolumeID: "volume-a",
		Digest: "sha256:seed", SizeBytes: uint64(len("seed-content")),
		InputDigest: seed.InputDigest(), ObservedGeneration: projected.Metadata().Version().String(), Phase: hypervisor.CloudInitPhaseNotReady,
	}
	s.Create(asset)
	// An unfinished seed is not attachable.
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](s, "guest")

	ctest.UpdateWithConflicts(s, asset, func(current *hypervisor.CloudInitStatus) error {
		current.TypedSpec().Phase = hypervisor.CloudInitPhaseReady

		return nil
	})

	ctest.AssertResource(s, "guest", func(domain *hypervisor.VirtualMachineDomainSpec, a *assert.Assertions) {
		a.Contains(domain.TypedSpec().DomainXML, `device="cdrom"`)
		a.Contains(domain.TypedSpec().DomainXML, `bus="sata"`)
		a.Contains(domain.TypedSpec().DomainXML, `<readonly></readonly>`)
		a.Contains(domain.TypedSpec().DomainXML, path)
		a.Equal(id, domain.TypedSpec().CloudInit)
		a.Equal("running", domain.TypedSpec().PowerState)
	})
	ctest.UpdateWithConflicts(s, pool, func(current *storage.StoragePoolStatus) error {
		current.TypedSpec().VolumeID = "volume-b"
		current.TypedSpec().Phase = storage.StoragePoolPhaseNotReady

		return nil
	})
	ctest.AssertResource(s, "guest", func(domain *hypervisor.VirtualMachineDomainSpec, a *assert.Assertions) {
		a.Equal("stopped", domain.TypedSpec().PowerState)
	})
}

func (s *VirtualMachineCloudInitDomainSuite) TestPoolSeedRendersAfterLibraryDisappears() {
	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest")
	vm.TypedSpec().CPU.Count = 1
	vm.TypedSpec().Memory.Size = 512 << 20
	vm.TypedSpec().PowerState = "running"
	vm.TypedSpec().Firmware.Type = "bios"
	vm.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{Pool: "pool-a", UserData: "#cloud-config\n"}
	dir := s.T().TempDir()
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool-a")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	pool.TypedSpec().TargetPath = dir
	pool.TypedSpec().VolumeID = "pool-volume"
	s.Create(pool)
	s.Create(vm)

	seed := hypervisor.CloudInitSpecSpec{Pool: "pool-a", UserData: "#cloud-config\n"}
	id := hypervisor.CloudInitStatusID("guest", seed)

	ctest.AssertResource(s, "guest", func(projected *hypervisor.CloudInitSpec, a *assert.Assertions) {
		a.Equal(seed, *projected.TypedSpec())
	})
	projected, err := safe.StateGetByID[*hypervisor.CloudInitSpec](s.Ctx(), s.State(), "guest")
	s.Require().NoError(err)

	name := "seed.iso"
	path := filepath.Join(dir, name)
	s.Require().NoError(os.WriteFile(path, []byte("seed"), 0o600))

	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, storage.StoragePoolVolumeID("pool-a", name))
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{Pool: "pool-a", Name: name, Path: path, Format: "raw", Capacity: 4, Phase: storage.StoragePoolVolumePhaseReady}
	s.Create(volume)

	asset := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id)
	*asset.TypedSpec() = hypervisor.CloudInitStatusSpec{
		VirtualMachine: "guest", Pool: "pool-a", Name: name, Path: dir,
		VolumeID:  "pool-volume",
		SizeBytes: 4, Digest: "sha256:seed", InputDigest: seed.InputDigest(), ObservedGeneration: projected.Metadata().Version().String(), Phase: hypervisor.CloudInitPhaseReady,
	}
	s.Create(asset)
	ctest.AssertResource(s, "guest", func(domain *hypervisor.VirtualMachineDomainSpec, a *assert.Assertions) {
		a.Contains(domain.TypedSpec().DomainXML, path)
		a.Equal("running", domain.TypedSpec().PowerState)
	})
	// A directly generated pool seed has no library dependency.
	lib := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	lib.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(lib)
	s.Destroy(lib)
	ctest.AssertResource(s, "guest", func(domain *hypervisor.VirtualMachineDomainSpec, a *assert.Assertions) {
		a.Contains(domain.TypedSpec().DomainXML, path)
		a.Equal("running", domain.TypedSpec().PowerState)
	})
}

func (s *VirtualMachineCloudInitDomainSuite) TestProducedSeedAttachesWithoutLibraryOrVolumeRequest() {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.CloudInitISOController{}))
	dir := s.T().TempDir()
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool-a")
	pool.TypedSpec().TargetPath = filepath.Join(dir, "pool-a")
	pool.TypedSpec().VolumeID = "pool-backing"
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Require().NoError(os.Mkdir(pool.TypedSpec().TargetPath, 0o700))

	mount := block.NewVolumeMountStatus(block.NamespaceName, "pool-backing")
	mount.TypedSpec().Target = dir
	s.Create(mount)
	s.Create(pool)

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest")
	vm.TypedSpec().CPU.Count = 1
	vm.TypedSpec().Memory.Size = 512 << 20
	vm.TypedSpec().Firmware.Type = "bios"
	vm.TypedSpec().PowerState = "running"
	vm.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{Pool: "pool-a", UserData: "#cloud-config\n"}
	s.Create(vm)
	ctest.AssertResource(s, "guest", func(domain *hypervisor.VirtualMachineDomainSpec, a *assert.Assertions) {
		a.Equal("running", domain.TypedSpec().PowerState)
		a.Contains(domain.TypedSpec().DomainXML, pool.TypedSpec().TargetPath+"/cloud-init-")
		a.Contains(domain.TypedSpec().DomainXML, "<readonly></readonly>")
		a.NotEmpty(domain.TypedSpec().CloudInit)
	})
	requests, err := safe.StateListAll[*storage.StoragePoolVolumeSpec](s.Ctx(), s.State())
	s.Require().NoError(err)
	s.Zero(requests.Len())
}

const blankVolumePath = "/storage-pool/booted__data.qcow2"

func (suite *VirtualMachineSpecSuite) TestRendersBlankDisk() {
	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, storage.StoragePoolVolumeID("pool1", "booted__data.qcow2"))
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     "pool1",
		Name:     "booted__data.qcow2",
		Path:     blankVolumePath,
		Format:   "qcow2",
		Capacity: 20 << 30,
		Phase:    storage.StoragePoolVolumePhaseReady,
	}
	suite.Create(volume, state.WithCreateOwner("storage.StoragePoolVolumeController"))

	doc := newVirtualMachine("booted")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:      "data",
			DiskPool:      "pool1",
			DiskSize:      meta.MustByteSize("20GiB"),
			DiskFormat:    hypervisorhelpers.VirtualMachineDiskFormatQCOW2,
			DiskBootOrder: 1,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{},
			},
		},
	}

	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	want, err := os.ReadFile(filepath.Join("testdata", "virtualmachinespec", "blank-disk.xml"))
	suite.Require().NoError(err)

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal(string(want), res.TypedSpec().DomainXML+"\n")
	})

	res, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), doc.Name())
	suite.Require().NoError(err)
	suite.Require().NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))
}
