// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// redactgen generates RedactSecrets methods for the structs marked with the //redactgen:gen comment.
//
// Sensitive fields are marked with the `redact:"..."` struct tag, and the generated code takes care
// of walking the struct (including the nested structs, slices and maps) to find them.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var (
		headerFile string
		output     string
	)

	flag.StringVar(&headerFile, "header-file", "", "path to the file with the boilerplate header")
	flag.StringVar(&output, "o", "redact.generated.go", "name of the generated file")
	flag.Parse()

	if err := run(flag.Args(), headerFile, output); err != nil {
		fmt.Fprintf(os.Stderr, "redactgen: %s\n", err)

		os.Exit(1)
	}
}

func run(args []string, headerFile, output string) error {
	if len(args) != 1 {
		return errors.New("exactly one package path is expected")
	}

	var header []byte

	if headerFile != "" {
		var err error

		if header, err = os.ReadFile(headerFile); err != nil {
			return fmt.Errorf("failed to read the header file: %w", err)
		}
	}

	pkg, err := loadPackage(args[0])
	if err != nil {
		return fmt.Errorf("%w\n\nhint: if %q is stale, delete it and re-run the generator", err, output)
	}

	source, err := generate(pkg, options{
		outputFile:  filepath.Base(output),
		header:      string(header),
		commandLine: strings.Join(append([]string{"redactgen"}, os.Args[1:]...), " "),
	})
	if err != nil {
		return err
	}

	if source == nil {
		// nothing is marked for generation in this package, don't leave a stale file behind
		if err = os.Remove(output); err != nil && !os.IsNotExist(err) {
			return err
		}

		return nil
	}

	return os.WriteFile(output, source, 0o644)
}
