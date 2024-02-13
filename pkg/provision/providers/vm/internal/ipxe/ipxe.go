// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ipxe provides utility to deliver iPXE images and build iPXE scripts.
package ipxe

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"text/template"
)

//go:embed "data/*"
var ipxeFiles embed.FS

// TFTPHandler is called when client starts file download from the TFTP server.
//
// TFTP handler also patches the iPXE binary on the fly with the script
// which chainloads next handler.
func TFTPHandler(next string) func(filename string, rf io.ReaderFrom) error {
	return func(filename string, rf io.ReaderFrom) error {
		log.Printf("tftp request: %s", filename)

		file, err := ipxeFiles.Open(filepath.Join("data", filename))
		if err != nil {
			return err
		}

		defer file.Close() //nolint:errcheck

		contents, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		var script bytes.Buffer

		if err = scriptTemplate.Execute(&script, struct {
			Next string
		}{
			Next: next,
		}); err != nil {
			return err
		}

		contents, err = patchScript(contents, script.Bytes())
		if err != nil {
			return fmt.Errorf("error patching %q: %w", filename, err)
		}

		_, err = rf.ReadFrom(bytes.NewReader(contents))

		return err
	}
}

// scriptTemplate to run DHCP and chain the boot to the .Next endpoint.
var scriptTemplate = template.Must(template.New("iPXE embedded").Parse(`#!ipxe
prompt --key 0x02 --timeout 2000 Press Ctrl-B for the iPXE command line... && shell ||

{{/* print interfaces */}}
ifstat

{{/* retry 10 times overall */}}
set attempts:int32 10
set x:int32 0

:retry_loop

	set idx:int32 0

	:loop
		{{/* try DHCP on each interface */}}
		isset ${net${idx}/mac} || goto exhausted

		ifclose
		iflinkwait --timeout 5000 net${idx} || goto next_iface
		dhcp net${idx} || goto next_iface
		goto boot

	:next_iface
		inc idx && goto loop

	:boot
		{{/* attempt boot, if fails try next iface */}}
		route

		chain --replace {{ .Next }} || goto next_iface

:exhausted
	echo
	echo Failed to iPXE boot successfully via all interfaces

	iseq ${x} ${attempts} && goto fail ||

	echo Retrying...
	echo

	inc x
	goto retry_loop

:fail
	echo
	echo Failed to get a valid response after ${attempts} attempts
	echo

	echo Rebooting in 5 seconds...
	sleep 5
	reboot
`))

var (
	placeholderStart = []byte("# *PLACEHOLDER START*")
	placeholderEnd   = []byte("# *PLACEHOLDER END*")
)

// patchScript patches the iPXE script into the iPXE binary.
//
// The iPXE binary should be built uncompressed with an embedded
// script stub which contains abovementioned placeholders.
func patchScript(contents, script []byte) ([]byte, error) {
	start := bytes.Index(contents, placeholderStart)
	if start == -1 {
		return nil, errors.New("placeholder start not found")
	}

	end := bytes.Index(contents, placeholderEnd)
	if end == -1 {
		return nil, errors.New("placeholder end not found")
	}

	if end < start {
		return nil, errors.New("placeholder end before start")
	}

	end += len(placeholderEnd)

	length := end - start

	if len(script) > length {
		return nil, fmt.Errorf("script size %d is larger than placeholder space %d", len(script), length)
	}

	script = append(script, bytes.Repeat([]byte{'\n'}, length-len(script))...)

	copy(contents[start:end], script)

	return contents, nil
}
