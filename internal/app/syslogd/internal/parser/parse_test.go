// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// package parser provides a syslog parser that can parse both RFC3164 and RFC5424 with best effort.
package parser_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/syslogd/internal/parser"
)

func TestParser(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "RFC3164 without tag and hostname",
			input:    []byte(`<4>Feb 16 17:54:19 time="2024-02-16T17:54:19.857755073Z" level=warning msg="Could not add /dev/mshv to the devices cgroup`),
			expected: `{"content":"msg=\"Could not add /dev/mshv to the devices cgroup","facility":0,"hostname":"time=\"2024-02-16T17:54:19.857755073Z\"","priority":4,"severity":4,"tag":"level=warning","timestamp":"2024-02-16T17:54:19Z"}`, //nolint:lll
		},
		{
			name:     "RFC3164 without hostname",
			input:    []byte(`<4>Feb 16 17:54:19 kata[2569]: time="2024-02-16T17:54:19.857755073Z" level=warning msg="Could not add /dev/mshv to the devices cgroup`),
			expected: `{"content":"time=\"2024-02-16T17:54:19.857755073Z\" level=warning msg=\"Could not add /dev/mshv to the devices cgroup","facility":0,"hostname":"localhost","priority":4,"severity":4,"tag":"kata","timestamp":"2024-02-16T17:54:19Z"}`, //nolint:lll
		},
		{
			name:     "RFC3164 with hostname",
			input:    []byte(`<4>Feb 16 17:54:19 hostname kata[2569]: time="2024-02-16T17:54:19.857755073Z" level=warning msg="Could not add /dev/mshv to the devices cgroup`),
			expected: `{"content":"time=\"2024-02-16T17:54:19.857755073Z\" level=warning msg=\"Could not add /dev/mshv to the devices cgroup","facility":0,"hostname":"hostname","priority":4,"severity":4,"tag":"kata","timestamp":"2024-02-16T17:54:19Z"}`, //nolint:lll
		},
		{
			name:     "RFC5424",
			input:    []byte(`<165>1 2003-10-11T22:14:15.003Z mymachine.example.com evntslog - ID47 [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"] An application event log entry...`),
			expected: `{"app_name":"evntslog","facility":20,"hostname":"mymachine.example.com","message":"An application event log entry...","msg_id":"ID47","priority":165,"proc_id":"-","severity":5,"structured_data":"[exampleSDID@32473 iut=\"3\" eventSource=\"Application\" eventID=\"1011\"]","timestamp":"2003-10-11T22:14:15.003Z","version":1}`, //nolint:lll
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsedJSON, err := parser.Parse(tc.input)
			require.NoError(t, err)

			require.Equal(t, tc.expected, parsedJSON)
		})
	}
}
