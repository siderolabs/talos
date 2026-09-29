// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/relaxng"
	"github.com/stretchr/testify/require"
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

	doc, err := helium.NewParser().Parse(context.Background(), data)
	if err != nil {
		return fmt.Errorf("parse domain XML: %w", err)
	}

	return relaxng.NewValidator(domainGrammar.grammar).Validate(context.Background(), doc)
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
