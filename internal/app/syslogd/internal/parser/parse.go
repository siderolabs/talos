// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package parser provides a syslog parser that can parse both RFC3164 and RFC5424 with best effort.
package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/jeromer/syslogparser"
	"github.com/jeromer/syslogparser/rfc3164"
	"github.com/jeromer/syslogparser/rfc5424"
)

// Parse parses a syslog message and returns a json encoded representation of the message.
// If an RFC3164 message is detected and there is no hostname field in the message,
// the hostname field is set to "localhost".
func Parse(b []byte) (string, error) {
	// Detect the RFC version
	rfc, err := syslogparser.DetectRFC(b)
	if err != nil {
		return "", err
	}

	var parser syslogparser.LogParser

	switch rfc {
	case syslogparser.RFC_3164:
		input := slices.Clone(b)

		tagPresent, hostnamePresent := rfc3164ContainsTagHostname(b)

		if !tagPresent {
			input = enhanceRFC3164WithTag(b)
		}

		parser = rfc3164.NewParser(input)

		if !hostnamePresent {
			parser.WithHostname("localhost")
		}
	case syslogparser.RFC_5424:
		parser = rfc5424.NewParser(b)
	default:
		return "", fmt.Errorf("unsupported RFC version: %v", rfc)
	}

	if err = parser.Parse(); err != nil {
		return "", err
	}

	msg, err := json.Marshal(parser.Dump())
	if err != nil {
		return "", err
	}

	return string(msg), nil
}

func rfc3164ContainsTagHostname(buf []byte) (bool, bool) {
	indx := bytes.Index(buf, []byte(`]:`))
	if indx == -1 {
		return false, false
	}

	// handle case when timestamp is of the format `<6>Mar  3 12:55:18`
	if len(bytes.Split(buf[:indx], []byte(`  `))) > 1 {
		return true, false
	}

	return true, bytes.Count(buf[:indx], []byte(` `)) > 3
}

func enhanceRFC3164WithTag(buf []byte) []byte {
	var count int

	spaces := 3

	singleDigitDayIndex := bytes.Index(buf, []byte(`  `))
	if singleDigitDayIndex != -1 && singleDigitDayIndex < 8 {
		spaces = 4
	}

	i := bytes.IndexFunc(buf, func(r rune) bool {
		if r == rune(' ') {
			count++
		}

		if count == spaces {
			return true
		}

		return false
	},
	)

	initial := buf[:i]
	remaining := buf[i:]

	var syslogBytes bytes.Buffer

	syslogBytes.Write(initial)
	syslogBytes.WriteString(" unknown:")
	syslogBytes.Write(remaining)

	return syslogBytes.Bytes()
}
