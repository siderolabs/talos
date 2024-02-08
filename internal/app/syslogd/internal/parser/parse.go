// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package parser provides a syslog parser that can parse both RFC3164 and RFC5424 with best effort.
package parser

import (
	"bytes"
	"encoding/json"
	"fmt"

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
		parser = rfc3164.NewParser(b)

		if rfc3164ContainsHostname(b) {
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

func rfc3164ContainsHostname(buf []byte) bool {
	indx := bytes.Index(buf, []byte(`]:`))
	if indx == -1 {
		return false
	}

	return bytes.Count(buf[:indx], []byte(` `)) == 3
}
