// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main builds the SPDX SBOM for a Talos build artifact.
//
// It wraps syft's library API to scan a directory and emit SPDX JSON, then
// post-processes the typed *spdx.Document to:
//
//  1. Add two synthetic SPDX packages as siblings of the directory-root
//     package syft emits, both linked via CONTAINS relationships:
//     - `Package-os-talos` (PrimaryPackagePurpose=OPERATING-SYSTEM) with
//     PURL `pkg:generic/talos@<tag>` (VEX product correlation for
//     `generate-vex`) and CPE `cpe:2.3:o:siderolabs:talos_linux:<ver>:*…`
//     (NVD-keyed advisories like CVE-2022-36103).
//     - `Package-go-siderolabs-talos` (PrimaryPackagePurpose=LIBRARY)
//     with PURL `pkg:golang/github.com/siderolabs/talos@<tag>` for the
//     GHSAs GitHub publishes against the talos Go-module path
//     (GHSA-g5p6-327m-3fxx, GHSA-jr8j-2jhp-m67v, GHSA-m38g-vww2-mvgx).
//     GHSAs can't carry CPEs, so these are otherwise invisible to
//     grype on a talos SBOM.
//
//     Two packages rather than one because syft's SPDX importer assigns
//     a single PURL per pkg.Package (multiple `purl` externalRefs collapse
//     to the first), so the golang PURL needs its own SPDX package to
//     survive import and get classified as pkg.Type=go-module.
//
//     The SBOM root package can't host these identifiers either: syft's
//     SPDX importer strips the root and turns it into the SBOM's source
//     metadata
//     (https://github.com/anchore/syft/blob/v1.44.0/syft/format/common/spdxhelpers/to_syft_model.go#L128),
//     so any externalRefs on it never reach the vulnerability matcher.
//
//  2. Enrich the Go-module packages syft catalogs: per-module license
//     discovery is enabled against the local module cache (GOMODCACHE), and
//     each go-module package gets its PackageDownloadLocation (module proxy
//     zip) and PackageHomePage (pkg.go.dev) filled in, both derived from the
//     module path and version. syft leaves these as NOASSERTION otherwise.
//
//  3. Provide deterministic output (RFC3339 CreationInfo.Created derived from
//     SOURCE_DATE_EPOCH, plus a UUIDv5 documentNamespace hashed from a stable
//     digest of the cataloged packages). This replaces the prior fork-only
//     env vars while https://github.com/anchore/syft/pull/3932 remains open.
//
// Once that upstream PR merges and the syft pin moves past it, the
// determinism shim can be removed; the OS-package injection stays.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/cataloging"
	"github.com/anchore/syft/syft/cataloging/pkgcataloging"
	"github.com/anchore/syft/syft/format/common/spdxhelpers"
	"github.com/anchore/syft/syft/pkg/cataloger/golang"
	"github.com/anchore/syft/syft/source"
	"github.com/google/uuid"
	"github.com/spdx/tools-golang/spdx/v2/common"
	v2_3 "github.com/spdx/tools-golang/spdx/v2/v2_3"
	"golang.org/x/mod/module"
	_ "modernc.org/sqlite" // pulled in by syft catalogers; harmless if unused
)

func main() {
	var (
		sourceDir       string
		sourceName      string
		sourceVersion   string
		cpeVendor       string
		cpeProduct      string
		sourceDateEpoch int64
		outputPath      string
	)

	flag.StringVar(&sourceDir, "source-dir", "", "path to the directory to scan (required)")
	flag.StringVar(&sourceName, "source-name", "", "logical name of the scanned product, e.g. \"talos\" (required)")
	flag.StringVar(&sourceVersion, "source-version", "", "version tag for the scanned product, e.g. \"v1.13.3\" (required)")
	flag.StringVar(&cpeVendor, "cpe-vendor", "siderolabs", "CPE vendor for the OS root package")
	flag.StringVar(&cpeProduct, "cpe-product", "talos_linux", "CPE product for the OS root package")
	flag.Int64Var(&sourceDateEpoch, "source-date-epoch", parseEpochFromEnv(), "unix timestamp for SBOM creation; defaults to SOURCE_DATE_EPOCH env or 0")
	flag.StringVar(&outputPath, "output", "", "path to write the SBOM (required)")
	flag.Parse()

	if sourceDir == "" || sourceName == "" || sourceVersion == "" || outputPath == "" {
		flag.Usage()

		os.Exit(1)
	}

	if err := run(sourceDir, sourceName, sourceVersion, cpeVendor, cpeProduct, sourceDateEpoch, outputPath); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run(sourceDir, sourceName, sourceVersion, cpeVendor, cpeProduct string, sourceDateEpoch int64, outputPath string) error {
	normalizedName := normalize(sourceName)
	cpeVersion := strings.TrimPrefix(sourceVersion, "v")

	ctx := context.Background()

	src, err := syft.GetSource(
		ctx, sourceDir,
		syft.DefaultGetSourceConfig().
			WithSources("dir").
			WithAlias(source.Alias{Name: normalizedName, Version: sourceVersion}),
	)
	if err != nil {
		return fmt.Errorf("get source: %w", err)
	}
	defer src.Close() //nolint:errcheck

	// Enable the Go cataloger's per-module license discovery from the local
	// module cache. syft's default mod-cache dir is derived from GOPATH, but the
	// Talos build sets GOMODCACHE explicitly (e.g. /.cache/mod), so point the
	// cataloger at GOMODCACHE when it is set. The cache is fully populated by the
	// preceding `go mod download`, so this stays offline and reproducible.
	goCfg := golang.DefaultCatalogerConfig().
		WithSearchLocalModCacheLicenses(true).
		WithLocalModCacheDir(os.Getenv("GOMODCACHE"))

	cfg := syft.DefaultCreateSBOMConfig().
		WithCatalogerSelection(
			cataloging.NewSelectionRequest().WithExpression("+sbom-cataloger,go"),
		).
		WithPackagesConfig(pkgcataloging.DefaultConfig().WithGolangConfig(goCfg))

	sbomDoc, err := syft.CreateSBOM(ctx, src, cfg)
	if err != nil {
		return fmt.Errorf("create SBOM: %w", err)
	}

	doc := spdxhelpers.ToFormatModel(*sbomDoc)

	addOSPackage(
		doc, normalizedName, sourceVersion,
		purlRef(normalizedName, sourceVersion),
		cpeRef(cpeVendor, cpeProduct, cpeVersion),
	)
	addGoModulePackage(doc, sourceVersion)

	enrichGoModuleURLs(doc)

	applyDeterminism(doc, normalizedName, sourceDateEpoch)

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer out.Close() //nolint:errcheck

	enc := json.NewEncoder(out)
	enc.SetIndent("", " ")

	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encode SBOM: %w", err)
	}

	return nil
}

// addOSPackage attaches an OPERATING-SYSTEM SPDX package carrying the
// supplied externalRefs as a sibling of the root directory package, plus a
// CONTAINS relationship from the root → the new package. Idempotent: if a
// package with the synthesized SPDXID already exists, nothing is added.
func addOSPackage(doc *v2_3.Document, name, version string, refs ...*v2_3.PackageExternalReference) {
	osID := common.ElementID(spdxIDSafe("Package-os-" + name))

	for _, p := range doc.Packages {
		if p.PackageSPDXIdentifier == osID {
			return
		}
	}

	osPkg := &v2_3.Package{
		PackageName:           name,
		PackageSPDXIdentifier: osID,
		PackageVersion:        version,
		PackageDownloadLocation: fmt.Sprintf(
			"https://github.com/siderolabs/talos/releases/tag/%s", version,
		),
		PackageSupplier: &common.Supplier{
			SupplierType: "Organization",
			Supplier:     "Sidero Labs, Inc. (https://siderolabs.com)",
		},
		PackageLicenseConcluded:   "MPL-2.0",
		PackageLicenseDeclared:    "MPL-2.0",
		PackageCopyrightText:      "Copyright Sidero Labs, Inc.",
		PrimaryPackagePurpose:     "OPERATING-SYSTEM",
		PackageExternalReferences: append([]*v2_3.PackageExternalReference{}, refs...),
	}

	doc.Packages = append(doc.Packages, osPkg)

	// Find the root SPDXID so we can declare the relationship.
	var rootID common.ElementID

	for _, p := range doc.Packages {
		if strings.HasPrefix(string(p.PackageSPDXIdentifier), "DocumentRoot-Directory-") {
			rootID = p.PackageSPDXIdentifier

			break
		}
	}

	if rootID != "" {
		doc.Relationships = append(
			doc.Relationships, &v2_3.Relationship{
				RefA:         common.MakeDocElementID("", string(rootID)),
				Relationship: common.TypeRelationshipContains,
				RefB:         common.MakeDocElementID("", string(osID)),
			},
		)
	}
}

// spdxIDSafe returns an SPDXID-safe slug (alphanumeric + dot/hyphen).
func spdxIDSafe(s string) string {
	var b strings.Builder

	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	return b.String()
}

func purlRef(name, version string) *v2_3.PackageExternalReference {
	return &v2_3.PackageExternalReference{
		Category: "PACKAGE-MANAGER",
		RefType:  "purl",
		Locator:  fmt.Sprintf("pkg:generic/%s@%s", name, version),
	}
}

func cpeRef(vendor, product, version string) *v2_3.PackageExternalReference {
	return &v2_3.PackageExternalReference{
		Category: "SECURITY",
		RefType:  "cpe23Type",
		Locator:  fmt.Sprintf("cpe:2.3:o:%s:%s:%s:*:*:*:*:*:*:*", vendor, product, version),
	}
}

// addGoModulePackage attaches a second sibling SPDX package whose PURL is
// `pkg:golang/github.com/siderolabs/talos@<version>` so grype's
// `github:language:go` matcher fires on talos-OS GHSAs.
//
// Why a separate package instead of a second externalRef on Package-os-talos:
// syft's SPDX importer (used by grype) maps each spdx.Package to a single
// pkg.Package with one PURL — multiple `purl` externalRefs on the same SPDX
// package collapse to the first, dropping the others. Splitting the golang
// PURL onto its own SPDX package preserves it and lets syft set
// `pkg.Type = "go-module"` from the `pkg:golang/...` prefix, which is what
// grype's go-language matcher checks.
//
// Why we need this PURL at all: GHSA advisories for Talos-OS vulnerabilities
// (GHSA-g5p6-327m-3fxx runc escape, GHSA-jr8j-2jhp-m67v nftables,
// GHSA-m38g-vww2-mvgx privesc — see
// https://github.com/advisories?query=siderolabs%2Ftalos) are typed as
// `ecosystem: GO, name: github.com/siderolabs/talos`. The OSV schema GitHub
// uses for GHSAs is ecosystem-keyed and has no CPE field, so the only way
// for grype to surface these advisories against a talos SBOM is via a
// golang-ecosystem PURL with the matching module path.
//
// Idempotent: if a package with the synthesized SPDXID already exists,
// nothing is added.
func addGoModulePackage(doc *v2_3.Document, version string) {
	const (
		modulePath = "github.com/siderolabs/talos"
		spdxID     = "Package-go-siderolabs-talos"
	)

	id := common.ElementID(spdxID)

	for _, p := range doc.Packages {
		if p.PackageSPDXIdentifier == id {
			return
		}
	}

	pkgEntry := &v2_3.Package{
		PackageName:           modulePath,
		PackageSPDXIdentifier: id,
		PackageVersion:        version,
		PackageDownloadLocation: fmt.Sprintf(
			"https://github.com/siderolabs/talos/releases/tag/%s", version,
		),
		PackageSupplier: &common.Supplier{
			SupplierType: "Organization",
			Supplier:     "Sidero Labs, Inc. (https://siderolabs.com)",
		},
		PackageLicenseConcluded: "MPL-2.0",
		PackageLicenseDeclared:  "MPL-2.0",
		PackageCopyrightText:    "Copyright Sidero Labs, Inc.",
		PrimaryPackagePurpose:   "LIBRARY",
		PackageExternalReferences: []*v2_3.PackageExternalReference{
			{
				Category: "PACKAGE-MANAGER",
				RefType:  "purl",
				Locator:  fmt.Sprintf("pkg:golang/%s@%s", modulePath, version),
			},
		},
	}
	doc.Packages = append(doc.Packages, pkgEntry)

	for _, p := range doc.Packages {
		if strings.HasPrefix(string(p.PackageSPDXIdentifier), "DocumentRoot-Directory-") {
			doc.Relationships = append(doc.Relationships, &v2_3.Relationship{
				RefA:         common.MakeDocElementID("", string(p.PackageSPDXIdentifier)),
				Relationship: common.TypeRelationshipContains,
				RefB:         common.MakeDocElementID("", string(id)),
			})

			break
		}
	}
}

const goProxyBaseURL = "https://proxy.golang.org"

// enrichGoModuleURLs fills in PackageDownloadLocation and PackageHomePage for
// every Go-module package syft cataloged. syft leaves both as NOASSERTION for
// go-module packages, but they are fully derivable from the module path and
// version:
//
//   - PackageDownloadLocation: the module proxy zip
//     (https://proxy.golang.org/<escaped-path>/@v/<escaped-version>.zip), the
//     canonical, content-addressed artifact for the module version.
//   - PackageHomePage: the pkg.go.dev landing page
//     (https://pkg.go.dev/<path>@<version>), the human-facing entry point.
//
// A package is treated as a Go module when it carries a `pkg:golang/...` purl
// externalRef. The module path is taken from PackageName (syft sets it to the
// module path) rather than parsed back out of the purl, so we avoid the
// purl-escaping round-trip; the proxy escaping is applied here via
// golang.org/x/mod/module.
//
// Only empty/NOASSERTION/NONE fields are filled, so identifiers already set by
// addOSPackage/addGoModulePackage (e.g. the GitHub release URL on the talos
// module) are preserved. Packages without a resolvable module version — local
// `replace` targets and the synthetic stdlib package, which has no dot-bearing
// module domain — are skipped, since neither the proxy nor pkg.go.dev can
// address them.
func enrichGoModuleURLs(doc *v2_3.Document) {
	for _, p := range doc.Packages {
		if !isGoModulePackage(p) {
			continue
		}

		modulePath := p.PackageName
		version := p.PackageVersion

		// Need a module-domain path (contains a dot) and a concrete version to
		// build either URL; local replaces and stdlib have neither.
		if version == "" || !strings.Contains(modulePath, ".") {
			continue
		}

		if isUnsetLocation(p.PackageDownloadLocation) {
			if escPath, err := module.EscapePath(modulePath); err == nil {
				if escVer, err := module.EscapeVersion(version); err == nil {
					p.PackageDownloadLocation = fmt.Sprintf(
						"%s/%s/@v/%s.zip", goProxyBaseURL, escPath, escVer,
					)
				}
			}
		}

		if p.PackageHomePage == "" {
			p.PackageHomePage = fmt.Sprintf("https://pkg.go.dev/%s@%s", modulePath, version)
		}
	}
}

// isGoModulePackage reports whether the SPDX package carries a `pkg:golang/...`
// purl externalRef, syft's marker for a Go-module package.
func isGoModulePackage(p *v2_3.Package) bool {
	for _, ref := range p.PackageExternalReferences {
		if ref.RefType == "purl" && strings.HasPrefix(ref.Locator, "pkg:golang/") {
			return true
		}
	}

	return false
}

// isUnsetLocation reports whether an SPDX download-location field carries no
// real value (empty, NOASSERTION, or NONE) and so is safe to fill in.
func isUnsetLocation(loc string) bool {
	return loc == "" || loc == "NOASSERTION" || loc == "NONE"
}

// applyDeterminism overwrites creationInfo.created and documentNamespace with
// reproducible values: the timestamp from epoch, and a documentNamespace whose
// UUID is derived from the document's identifying content.
//
// Created is only overridden when a positive epoch was supplied (i.e.
// SOURCE_DATE_EPOCH was set). With no epoch (0), syft's own timestamp is kept
// rather than stamping 1970-01-01. The namespace UUID is always recomputed,
// since it is content-derived and independent of the clock.
//
// Provisional shim until anchore/syft#3932 merges. Upstream syft v1.44.0's
// ToFormatModel takes no deterministic-UUID flag, so we reproduce #3932's
// algorithm here over the final document (after our OS/Go package injection).
// Delete this once #3932 is released and ToFormatModel computes the namespace
// natively.
func applyDeterminism(doc *v2_3.Document, sourceName string, epoch int64) {
	if doc.CreationInfo == nil {
		doc.CreationInfo = &v2_3.CreationInfo{}
	}

	if epoch > 0 {
		doc.CreationInfo.Created = time.Unix(epoch, 0).UTC().Format(time.RFC3339)
	}

	doc.DocumentNamespace = buildNamespace(sourceName, documentUUID(sourceName, doc))
}

// buildNamespace returns an SPDX documentNamespace in syft's
// "anchore.com/syft/dir/<name>-<uuid>" shape.
func buildNamespace(sourceName, uniqueID string) string {
	return fmt.Sprintf("https://anchore.com/syft/dir/%s-%s", sourceName, uniqueID)
}

// documentUUID derives the deterministic document UUID exactly as
// anchore/syft#3932 does (to_format_model.go, gated on deterministicUUID):
// uuid.NewSHA1 over uuid.NameSpaceOID seeded with the source identity, then
// every package and file SPDX identifier, every relationship, and every
// extracted-license id. Re-scanning the same source yields the same UUID;
// distinct SBOMs (talos-amd64 vs talos-arm64, which differ in their package,
// file and relationship sets) yield distinct UUIDs.
//
// sourceName stands in for syft's Source.ID, which is not carried on the SPDX
// document. Packages are sorted by SPDX identifier (mirroring #3932's
// Packages.Sorted()); files, relationships and licenses are taken in syft's
// deterministic emission order.
//
// https://github.com/anchore/syft/pull/3932
func documentUUID(sourceName string, doc *v2_3.Document) string {
	data := []byte(sourceName)

	pkgIDs := make([]string, 0, len(doc.Packages))
	for _, p := range doc.Packages {
		pkgIDs = append(pkgIDs, string(p.PackageSPDXIdentifier))
	}

	sort.Strings(pkgIDs)

	for _, id := range pkgIDs {
		data = append(data, id...)
	}

	for _, f := range doc.Files {
		data = append(data, string(f.FileSPDXIdentifier)...)
	}

	for _, r := range doc.Relationships {
		data = append(data, string(r.RefA.ElementRefID)...)
		data = append(data, string(r.RefB.ElementRefID)...)
		data = append(data, r.Relationship...)
	}

	for _, ol := range doc.OtherLicenses {
		data = append(data, ol.LicenseIdentifier...)
	}

	return uuid.NewSHA1(uuid.NameSpaceOID, data).String()
}

func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), "-"))
}

func parseEpochFromEnv() int64 {
	v := os.Getenv("SOURCE_DATE_EPOCH")
	if v == "" {
		return 0
	}

	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}

	return n
}
