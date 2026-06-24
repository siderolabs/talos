// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"crypto/fips140"
	"testing"
	"time"

	"github.com/spdx/tools-golang/spdx/v2/common"
	v2_3 "github.com/spdx/tools-golang/spdx/v2/v2_3"
	"github.com/stretchr/testify/assert"
)

// skipIfFIPS skips tests that exercise documentUUID, which derives a UUIDv5 via
// uuid.NewSHA1. SHA-1 is rejected (panics) under GODEBUG=fips140=only, the mode
// the CI unit-test job runs in. The namespace is metadata, not a security
// primitive, so the SHA-1 use is acceptable in non-FIPS builds.
func skipIfFIPS(t *testing.T) {
	t.Helper()

	if fips140.Enabled() {
		t.Skip("documentUUID uses crypto/sha1 (UUIDv5), disallowed under FIPS 140-only mode")
	}
}

func TestApplyDeterminism_CreatedGatedOnEpoch(t *testing.T) {
	skipIfFIPS(t)

	// epoch 0 (no SOURCE_DATE_EPOCH): keep syft's existing Created, never 1970.
	doc := newDoc()
	applyDeterminism(doc, "talos", 0)

	if doc.CreationInfo.Created != "2026-01-01T00:00:00Z" {
		t.Errorf("epoch=0 must preserve Created, got %q", doc.CreationInfo.Created)
	}

	// Positive epoch: stamp it deterministically.
	doc = newDoc()
	applyDeterminism(doc, "talos", 1700000000)

	if want := time.Unix(1700000000, 0).UTC().Format(time.RFC3339); doc.CreationInfo.Created != want {
		t.Errorf("Created = %q, want %q", doc.CreationInfo.Created, want)
	}

	// Nil CreationInfo must not panic and must be initialized.
	bare := &v2_3.Document{}
	applyDeterminism(bare, "talos", 0)

	if bare.CreationInfo == nil {
		t.Error("CreationInfo should be initialized when nil")
	}
}

func TestAddOSPackage_AddsSiblingWithRefs(t *testing.T) {
	doc := newDoc()

	addOSPackage(
		doc, "talos", "v1.13.3",
		purlRef("talos", "v1.13.3"),
		cpeRef("siderolabs", "talos_linux", "1.13.3"),
	)

	osPkg := findPackage(doc, "Package-os-talos")
	assert.NotNil(t, "expected Package-os-talos to be added, got: %v", packageIDs(doc))

	if osPkg.PrimaryPackagePurpose != "OPERATING-SYSTEM" {
		t.Errorf("PrimaryPackagePurpose = %q, want OPERATING-SYSTEM", osPkg.PrimaryPackagePurpose)
	}

	wantRefs := []string{
		"pkg:generic/talos@v1.13.3",
		"cpe:2.3:o:siderolabs:talos_linux:1.13.3:*:*:*:*:*:*:*",
	}
	if got := len(osPkg.PackageExternalReferences); got != len(wantRefs) {
		t.Fatalf("externalRefs len = %d, want %d", got, len(wantRefs))
	}

	for i, want := range wantRefs {
		if got := osPkg.PackageExternalReferences[i].Locator; got != want {
			t.Errorf("externalRefs[%d].Locator = %q, want %q", i, got, want)
		}
	}

	if got := len(doc.Relationships); got != 1 {
		t.Fatalf("relationships len = %d, want 1", got)
	}

	rel := doc.Relationships[0]
	if rel.Relationship != "CONTAINS" {
		t.Errorf("relationship type = %q, want CONTAINS", rel.Relationship)
	}
}

func TestAddGoModulePackage(t *testing.T) {
	doc := newDoc()

	addGoModulePackage(doc, "v1.13.3")

	goPkg := findPackage(doc, "Package-go-siderolabs-talos")
	if goPkg == nil {
		t.Fatalf("expected Package-go-siderolabs-talos to be added, got: %v", packageIDs(doc))
	}

	if goPkg.PackageName != "github.com/siderolabs/talos" {
		t.Errorf("name = %q", goPkg.PackageName)
	}

	if goPkg.PrimaryPackagePurpose != "LIBRARY" {
		t.Errorf("PrimaryPackagePurpose = %q, want LIBRARY", goPkg.PrimaryPackagePurpose)
	}

	if got := len(goPkg.PackageExternalReferences); got != 1 {
		t.Fatalf("externalRefs len = %d, want 1", got)
	}

	wantLocator := "pkg:golang/github.com/siderolabs/talos@v1.13.3"
	if goPkg.PackageExternalReferences[0].Locator != wantLocator {
		t.Errorf("purl = %q, want %q", goPkg.PackageExternalReferences[0].Locator, wantLocator)
	}
}

func TestAddGoModulePackage_Idempotent(t *testing.T) {
	doc := newDoc()

	addGoModulePackage(doc, "v1.13.3")
	addGoModulePackage(doc, "v1.13.3")

	count := 0

	for _, p := range doc.Packages {
		if p.PackageSPDXIdentifier == "Package-go-siderolabs-talos" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("count = %d, want 1 (idempotent)", count)
	}
}

func TestAddOSPackage_Idempotent(t *testing.T) {
	doc := newDoc()

	addOSPackage(doc, "talos", "v1.13.3", purlRef("talos", "v1.13.3"))
	addOSPackage(doc, "talos", "v1.13.3", purlRef("talos", "v1.13.3"))

	osPkgs := 0

	for _, p := range doc.Packages {
		if p.PackageSPDXIdentifier == "Package-os-talos" {
			osPkgs++
		}
	}

	if osPkgs != 1 {
		t.Errorf("Package-os-talos count = %d, want 1 (idempotent)", osPkgs)
	}

	if got := len(doc.Relationships); got != 1 {
		t.Errorf("relationships count = %d, want 1 (idempotent)", got)
	}
}

func TestBuildNamespace_Shape(t *testing.T) {
	// buildNamespace is a pure formatter: prefix + name + "-" + uniqueID.
	const uniqueID = "11111111-2222-5333-8444-555555555555"

	got := buildNamespace("talos", uniqueID)

	const want = "https://anchore.com/syft/dir/talos-" + uniqueID
	if got != want {
		t.Errorf("buildNamespace = %q, want %q", got, want)
	}
}

func TestDocumentUUID_VersionFive(t *testing.T) {
	skipIfFIPS(t)

	// UUID format: XXXXXXXX-XXXX-VXXX-XXXX-XXXXXXXXXXXX; version digit at index 14.
	id := documentUUID("talos", &v2_3.Document{
		Packages: []*v2_3.Package{{PackageSPDXIdentifier: "P-a"}},
	})

	if got := id[14:15]; got != "5" {
		t.Errorf("UUID version digit = %q, want \"5\" (RFC 4122 SHA1); uuid=%q", got, id)
	}
}

func TestSpdxIDSafe(t *testing.T) {
	cases := map[string]string{
		"talos":                "talos",
		"Talos-v1":             "Talos-v1",
		"with spaces":          "with-spaces",
		"under_scores":         "under-scores",
		"dots.and-dashes-1.13": "dots.and-dashes-1.13",
	}
	for in, want := range cases {
		if got := spdxIDSafe(in); got != want {
			t.Errorf("spdxIDSafe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDocumentUUID_StableAcrossPackageOrder(t *testing.T) {
	skipIfFIPS(t)

	docA := &v2_3.Document{Packages: []*v2_3.Package{
		{PackageSPDXIdentifier: "P-a", PackageName: "a", PackageVersion: "1"},
		{PackageSPDXIdentifier: "P-b", PackageName: "b", PackageVersion: "2"},
	}}
	docB := &v2_3.Document{Packages: []*v2_3.Package{
		{PackageSPDXIdentifier: "P-b", PackageName: "b", PackageVersion: "2"},
		{PackageSPDXIdentifier: "P-a", PackageName: "a", PackageVersion: "1"},
	}}

	if documentUUID("talos", docA) != documentUUID("talos", docB) {
		t.Error("UUID should be independent of package order")
	}
}

// TestDocumentUUID_DistinguishesContent guards the collision fix: per
// anchore/syft#3932 the namespace UUID hashes files and relationships (and the
// source name), not just the package list. talos-amd64 and talos-arm64 share
// package names/versions but differ in files and relationships, so they must
// get distinct namespaces.
func TestDocumentUUID_DistinguishesContent(t *testing.T) {
	skipIfFIPS(t)

	base := func() *v2_3.Document {
		return &v2_3.Document{Packages: []*v2_3.Package{
			{PackageSPDXIdentifier: "P-a", PackageName: "a", PackageVersion: "1"},
			{PackageSPDXIdentifier: "P-b", PackageName: "b", PackageVersion: "2"},
		}}
	}

	ref := documentUUID("talos", base())

	withFile := base()
	withFile.Files = []*v2_3.File{{FileSPDXIdentifier: "File-x"}}

	if documentUUID("talos", withFile) == ref {
		t.Error("differing files must change the UUID")
	}

	withRel := base()
	withRel.Relationships = []*v2_3.Relationship{{
		RefA:         common.DocElementID{ElementRefID: "P-a"},
		RefB:         common.DocElementID{ElementRefID: "P-b"},
		Relationship: common.TypeRelationshipContains,
	}}

	if documentUUID("talos", withRel) == ref {
		t.Error("differing relationships must change the UUID")
	}

	if documentUUID("talos-enterprise", base()) == ref {
		t.Error("differing source name must change the UUID")
	}
}

func TestEnrichGoModuleURLs(t *testing.T) {
	goRef := func(locator string) *v2_3.PackageExternalReference {
		return &v2_3.PackageExternalReference{Category: "PACKAGE-MANAGER", RefType: "purl", Locator: locator}
	}

	doc := &v2_3.Document{Packages: []*v2_3.Package{
		// plain go module: both URLs get filled in
		{
			PackageSPDXIdentifier:     "P-mod",
			PackageName:               "github.com/foo/Bar",
			PackageVersion:            "v1.2.3",
			PackageDownloadLocation:   "NOASSERTION",
			PackageExternalReferences: []*v2_3.PackageExternalReference{goRef("pkg:golang/github.com/foo/Bar@v1.2.3")},
		},
		// existing download location is preserved; only homepage is added
		{
			PackageSPDXIdentifier:     "P-talos",
			PackageName:               "github.com/siderolabs/talos",
			PackageVersion:            "v1.13.3",
			PackageDownloadLocation:   "https://github.com/siderolabs/talos/releases/tag/v1.13.3",
			PackageExternalReferences: []*v2_3.PackageExternalReference{goRef("pkg:golang/github.com/siderolabs/talos@v1.13.3")},
		},
		// stdlib: no module domain, left untouched
		{
			PackageSPDXIdentifier:     "P-std",
			PackageName:               "stdlib",
			PackageVersion:            "go1.26.4",
			PackageDownloadLocation:   "NOASSERTION",
			PackageExternalReferences: []*v2_3.PackageExternalReference{goRef("pkg:golang/stdlib@go1.26.4")},
		},
		// local replace target: no version, left untouched
		{
			PackageSPDXIdentifier:     "P-local",
			PackageName:               "github.com/foo/local",
			PackageVersion:            "",
			PackageDownloadLocation:   "NOASSERTION",
			PackageExternalReferences: []*v2_3.PackageExternalReference{goRef("pkg:golang/github.com/foo/local")},
		},
		// non-go package: untouched
		{
			PackageSPDXIdentifier:   "P-other",
			PackageName:             "libfoo",
			PackageVersion:          "1.0",
			PackageDownloadLocation: "NOASSERTION",
		},
	}}

	enrichGoModuleURLs(doc)

	mod := findPackage(doc, "P-mod")
	// uppercase Bar must be proxy-escaped to !bar
	if want := "https://proxy.golang.org/github.com/foo/!bar/@v/v1.2.3.zip"; mod.PackageDownloadLocation != want {
		t.Errorf("download location = %q, want %q", mod.PackageDownloadLocation, want)
	}

	if want := "https://pkg.go.dev/github.com/foo/Bar@v1.2.3"; mod.PackageHomePage != want {
		t.Errorf("homepage = %q, want %q", mod.PackageHomePage, want)
	}

	talos := findPackage(doc, "P-talos")
	if want := "https://github.com/siderolabs/talos/releases/tag/v1.13.3"; talos.PackageDownloadLocation != want {
		t.Errorf("existing download location overwritten: got %q", talos.PackageDownloadLocation)
	}

	if want := "https://pkg.go.dev/github.com/siderolabs/talos@v1.13.3"; talos.PackageHomePage != want {
		t.Errorf("talos homepage = %q, want %q", talos.PackageHomePage, want)
	}

	for _, id := range []common.ElementID{"P-std", "P-local", "P-other"} {
		p := findPackage(doc, id)
		if p.PackageDownloadLocation != "NOASSERTION" || p.PackageHomePage != "" {
			t.Errorf("%s should be untouched, got download=%q homepage=%q", id, p.PackageDownloadLocation, p.PackageHomePage)
		}
	}
}

// helpers

func newDoc() *v2_3.Document {
	return &v2_3.Document{
		CreationInfo:      &v2_3.CreationInfo{Created: "2026-01-01T00:00:00Z"},
		DocumentNamespace: "https://anchore.com/syft/dir/talos-00000000-0000-0000-0000-000000000000",
		Packages: []*v2_3.Package{
			{
				PackageSPDXIdentifier: "DocumentRoot-Directory-talos",
				PackageName:           "talos",
				PackageVersion:        "v1.13.3",
				PrimaryPackagePurpose: "FILE",
			},
		},
	}
}

func findPackage(doc *v2_3.Document, id common.ElementID) *v2_3.Package {
	for _, p := range doc.Packages {
		if p.PackageSPDXIdentifier == id {
			return p
		}
	}

	return nil
}

func packageIDs(doc *v2_3.Document) []common.ElementID {
	ids := make([]common.ElementID, 0, len(doc.Packages))
	for _, p := range doc.Packages {
		ids = append(ids, p.PackageSPDXIdentifier)
	}

	return ids
}
