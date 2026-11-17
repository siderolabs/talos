// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/firmware/lvfs"
)

const (
	testGUID     = "11111111-2222-3333-4444-555555555555"
	testProtocol = "org.nvmexpress"
	testCHID     = "6de5d951-d755-576b-bd09-c5cf66b27234"
)

// component renders a one-component catalog with the given version format,
// release versions (most recently published first) and requirements.
func component(format string, versions []string, requires string) string {
	var releases strings.Builder

	for i, version := range versions {
		fmt.Fprintf(&releases, `<release version=%q timestamp="%d"><location>%s.cab</location></release>`, version, len(versions)-i, version)
	}

	return fmt.Sprintf(`<components origin="lvfs" version="0.9"><component type="firmware">
<id>com.example.nvme</id>
<provides><firmware type="flashed">%s</firmware></provides>
<custom><value key="LVFS::VersionFormat">%s</value><value key="LVFS::UpdateProtocol">%s</value></custom>
<releases>%s</releases>
<requires>%s</requires>
</component></components>`, testGUID, format, testProtocol, releases.String(), requires)
}

func catalogOf(t *testing.T, xml string) *lvfs.Catalog {
	t.Helper()

	catalog, err := lvfs.ParseCatalog(strings.NewReader(xml), nil)
	require.NoError(t, err)

	return catalog
}

func TestUpdates(t *testing.T) {
	t.Parallel()

	device := lvfs.Device{
		GUIDs:     []string{"99999999-0000-0000-0000-000000000000", testGUID},
		Protocol:  testProtocol,
		Version:   "20200005",
		VendorIDs: []string{"NVME:0x15B7"},
	}

	onHardware := device
	onHardware.HardwareIDs = []string{"bbbbbbbb-0000-0000-0000-000000000000", testCHID}

	plain := []string{"20200012", "20200005", "20200001"}

	for _, tc := range []struct {
		name     string
		catalog  string
		device   lvfs.Device
		expected string // version of the update, empty for none
	}{
		{"newer release", component("plain", plain, ""), device, "20200012"},
		{"up to date", component("plain", plain, ""), lvfs.Device{GUIDs: device.GUIDs, Protocol: testProtocol, Version: "20200012"}, ""},
		{"ahead of catalog", component("plain", plain, ""), lvfs.Device{GUIDs: device.GUIDs, Protocol: testProtocol, Version: "20200020"}, ""},
		{"other protocol", component("plain", plain, ""), lvfs.Device{GUIDs: device.GUIDs, Protocol: "org.uefi.capsule", Version: "1"}, ""},
		{"other GUID", component("plain", plain, ""), lvfs.Device{GUIDs: []string{"99999999-0000-0000-0000-000000000000"}, Protocol: testProtocol, Version: "1"}, ""},
		{"no releases", component("plain", nil, ""), device, ""},
		{"vendor regex", component("plain", plain, `<firmware compare="regex" version="NVME:0x1C58|NVME:0x15B7">vendor-id</firmware>`), device, "20200012"},
		{"vendor eq", component("plain", plain, `<firmware compare="eq" version="NVME:0x15B7">vendor-id</firmware>`), device, "20200012"},
		{"vendor mismatch", component("plain", plain, `<firmware compare="eq" version="NVME:0x1344">vendor-id</firmware>`), device, ""},
		{"version ge", component("plain", plain, `<firmware compare="ge" version="20200001"/>`), device, "20200012"},
		{"version ge unmet", component("plain", plain, `<firmware compare="ge" version="20200010"/>`), device, ""},
		{"version glob", component("plain", plain, `<firmware compare="glob" version="2020*5"/>`), device, "20200012"},
		{"version glob unmet", component("plain", plain, `<firmware compare="glob" version="2021*"/>`), device, ""},
		{"version regex", component("plain", plain, `<firmware compare="regex" version="^2020000[0-9]$"/>`), device, "20200012"},
		{"version ne", component("plain", plain, `<firmware compare="ne" version="20200005"/>`), device, ""},
		{"version lt and gt", component("plain", plain, `<firmware compare="lt" version="20200006"/><firmware compare="gt" version="20200004"/>`), device, "20200012"},
		{"version le", component("plain", plain, `<firmware compare="le" version="20200004"/>`), device, ""},
		{"all requirements must hold", component("plain", plain, `<firmware compare="ge" version="20200001"/><firmware compare="eq" version="NVME:0x1344">vendor-id</firmware>`), device, ""},
		{"other firmware requirement", component("plain", plain, `<firmware compare="ge" version="1.0">bootloader</firmware>`), device, ""},
		{"unknown comparison", component("plain", plain, `<firmware compare="similar" version="20200005"/>`), device, ""},
		{"bad regex", component("plain", plain, `<firmware compare="regex" version="(">vendor-id</firmware>`), device, ""},
		{"parent firmware requirement", component("plain", plain, `<firmware compare="ge" version="20200001" depth="1"/>`), device, ""},
		{"own firmware requirement with depth", component("plain", plain, `<firmware compare="ge" version="20200001" depth="0"/>`), device, "20200012"},
		{"fwupd version is ignored", component("plain", plain, `<id compare="ge" version="1.2.2">org.freedesktop.fwupd</id>`), device, "20200012"},
		{"other software requirement", component("plain", plain, `<id compare="ge" version="0.3">org.freedesktop.gusb</id>`), device, ""},
		{"client requirement", component("plain", plain, `<client>update-action</client>`), device, ""},
		{"unknown requirement", component("plain", plain, `<something>else</something>`), device, ""},
		{"hardware unknown", component("plain", plain, `<hardware>`+testCHID+`</hardware>`), device, ""},
		{"hardware matches", component("plain", plain, `<hardware>aaaaaaaa-0000-0000-0000-000000000000|`+strings.ToUpper(testCHID)+`</hardware>`), onHardware, "20200012"},
		{"hardware differs", component("plain", plain, `<hardware>aaaaaaaa-0000-0000-0000-000000000000</hardware>`), onHardware, ""},
		{"every hardware requirement must hold", component("plain", plain, `<hardware>`+testCHID+`</hardware><hardware>aaaaaaaa-0000-0000-0000-000000000000</hardware>`), onHardware, ""},
		{"excluded hardware unknown", component("plain", plain, `<not_hardware>`+testCHID+`</not_hardware>`), device, ""},
		{"excluded hardware matches", component("plain", plain, `<not_hardware>`+testCHID+`</not_hardware>`), onHardware, ""},
		{"excluded hardware differs", component("plain", plain, `<not_hardware>aaaaaaaa-0000-0000-0000-000000000000</not_hardware>`), onHardware, "20200012"},
		{"unknown installed version", component("plain", plain, ""), lvfs.Device{GUIDs: device.GUIDs, Protocol: testProtocol}, ""},
		// a maintenance release of an old branch is the most recently published
		{"highest version, not latest published", component("plain", []string{"20200007", "20200012", "20200001"}, ""), device, "20200012"},
		{"late maintenance release is not an update", component("plain", []string{"20200003", "20200005"}, ""), device, ""},
		{"triplet newer", component("triplet", []string{"1.2.10", "1.2.9"}, ""), lvfs.Device{GUIDs: device.GUIDs, Protocol: testProtocol, Version: "1.2.9"}, "1.2.10"},
		{"triplet up to date", component("triplet", []string{"1.2.10"}, ""), lvfs.Device{GUIDs: device.GUIDs, Protocol: testProtocol, Version: "1.10.0"}, ""},
		{"format defaults to plain", component("", []string{"1.2.10", "1.2.9"}, ""), lvfs.Device{GUIDs: device.GUIDs, Protocol: testProtocol, Version: "1.2.9"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			updates := catalogOf(t, tc.catalog).Updates(tc.device)

			if tc.expected == "" {
				assert.Empty(t, updates)

				return
			}

			require.Len(t, updates, 1)
			assert.Equal(t, "com.example.nvme", updates[0].Component.ID)
			assert.Equal(t, tc.expected, updates[0].Release.Version)
		})
	}
}

func TestCatalogRelease(t *testing.T) {
	t.Parallel()

	device := lvfs.Device{
		GUIDs:     []string{testGUID},
		Protocol:  testProtocol,
		Version:   "20200005",
		VendorIDs: []string{"NVME:0x15B7"},
	}

	plain := []string{"20200012", "20200005", "20200001"}
	vendor := `<firmware compare="eq" version="NVME:0x15B7">vendor-id</firmware>`

	for _, tc := range []struct {
		name    string
		catalog string
		device  lvfs.Device
		version string
		found   bool
	}{
		{"newer, not the highest", component("plain", []string{"20200012", "20200007", "20200005"}, vendor), device, "20200007", true},
		{"highest", component("plain", plain, vendor), device, "20200012", true},
		{"older", component("plain", plain, vendor), device, "20200001", true},
		{"installed", component("plain", plain, vendor), device, "20200005", true},
		{"unpublished", component("plain", plain, vendor), device, "20200009", false},
		{"no partial match", component("plain", plain, vendor), device, "2020001", false},
		{"requirement unmet", component("plain", plain, `<firmware compare="eq" version="NVME:0x1344">vendor-id</firmware>`), device, "20200012", false},
		{"hardware restricted", component("plain", plain, `<hardware>`+testCHID+`</hardware>`), device, "20200012", false},
		{"unknown installed version", component("plain", plain, vendor), lvfs.Device{GUIDs: device.GUIDs, Protocol: testProtocol, VendorIDs: device.VendorIDs}, "20200012", true},
		{"unknown installed version, required", component("plain", plain, `<firmware compare="lt" version="20200012"/>`), lvfs.Device{GUIDs: device.GUIDs, Protocol: testProtocol}, "20200012", false},
		{"other protocol", component("plain", plain, vendor), lvfs.Device{GUIDs: device.GUIDs, Protocol: "org.uefi.capsule", Version: "1"}, "20200012", false},
		{"other GUID", component("plain", plain, vendor), lvfs.Device{GUIDs: []string{"99999999-0000-0000-0000-000000000000"}, Protocol: testProtocol, Version: "1"}, "20200012", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			found := catalogOf(t, tc.catalog).Release(tc.device, tc.version)

			if !tc.found {
				assert.Empty(t, found)

				return
			}

			require.Len(t, found, 1)
			assert.Equal(t, "com.example.nvme", found[0].Component.ID)
			assert.Equal(t, tc.version, found[0].Release.Version)
			assert.Equal(t, []string{tc.version + ".cab"}, found[0].Release.Locations)
		})
	}
}

func TestUpdatesMultipleComponents(t *testing.T) {
	t.Parallel()

	// two components provide the GUID; only the one whose vendor requirement
	// holds is offered
	xml := fmt.Sprintf(`<components origin="lvfs" version="0.9">
<component type="firmware"><id>com.example.other</id>
<provides><firmware type="flashed">%[1]s</firmware></provides>
<custom><value key="LVFS::UpdateProtocol">%[2]s</value></custom>
<releases><release version="2" timestamp="1"><location>2.cab</location></release></releases>
<requires><firmware compare="eq" version="NVME:0x1344">vendor-id</firmware></requires>
</component>
<component type="firmware"><id>com.example.nvme</id>
<provides><firmware type="flashed">%[1]s</firmware></provides>
<custom><value key="LVFS::UpdateProtocol">%[2]s</value></custom>
<releases><release version="3" timestamp="1"><location>3.cab</location></release></releases>
<requires><firmware compare="eq" version="NVME:0x15B7">vendor-id</firmware></requires>
</component></components>`, testGUID, testProtocol)

	updates := catalogOf(t, xml).Updates(lvfs.Device{
		GUIDs:     []string{testGUID},
		Protocol:  testProtocol,
		Version:   "1",
		VendorIDs: []string{"NVME:0x15B7"},
	})

	require.Len(t, updates, 1)
	assert.Equal(t, "com.example.nvme", updates[0].Component.ID)
	assert.Equal(t, "3", updates[0].Release.Version)
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		format   string
		a, b     string
		expected int
	}{
		{"plain", "620331WD", "624361WD", -1},
		{"plain", "624361WD", "620331WD", 1},
		{"plain", "620331WD", "620331WD", 0},
		{"plain", "1.10", "1.9", -1}, // strings, not numbers
		{"triplet", "1.10", "1.9", 1},
		{"triplet", "1.2.10", "1.2.9", 1},
		{"triplet", "1.2", "1.2.0", -1},
		{"quad", "1.2.3.4", "1.2.3.4", 0},
		{"triplet", "1.a", "1.b", -1},
		{"number", "10", "9", 1},
		{"hex", "0x10", "0x9", 1},
		{"hex", "0x0000000A", "0xa", 0},
		{"hex", "0x10", "17", -1},
		{"", "9", "10", -1},
	} {
		assert.Equal(t, tc.expected, lvfs.CompareVersions(tc.format, tc.a, tc.b), "%s: %q vs %q", tc.format, tc.a, tc.b)
	}
}

func TestVersionFormat(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "plain", (&lvfs.Component{}).VersionFormat())

	c := &lvfs.Component{Custom: []lvfs.CustomValue{{Key: "LVFS::VersionFormat", Value: " triplet "}}}
	assert.Equal(t, "triplet", c.VersionFormat())
}
