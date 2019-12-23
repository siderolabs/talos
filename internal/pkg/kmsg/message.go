// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kmsg

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Facility is an attribute of kernel log message.
type Facility int

// Kernel log facilities.
//
// From <sys/syslog.h>.
const (
	Kern Facility = iota
	User
	Mail
	Daemon
	Auth
	Syslog
	Lpr
	News
	Uucp
	Cron
	AuthPriv
	Local0
	Local1
	Local2
	Local3
	Local4
	Local5
	Local6
	Local7
)

func (f Facility) String() string {
	return [...]string{
		"kern", "user", "mail", "daemon",
		"auth", "syslog", "lpr", "news", "uucp",
		"cron", "authpriv",
		"local0", "local1", "local2", "local3",
		"local4", "local5", "local6", "local7",
	}[f]
}

// Priority is an attribute of kernel log message.
type Priority int

// Kernel log priorities.
const (
	Emerg Priority = iota
	Alert
	Crit
	Err
	Warning
	Notice
	Info
	Debug
)

func (p Priority) String() string {
	return [...]string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}[p]
}

// Message is a parsed kernel log message.
type Message struct {
	Facility       Facility
	Priority       Priority
	SequenceNumber int64
	Clock          int64
	Timestamp      time.Time
	Message        string
}

// ParseMessage parses internal kernel log format.
//
// Reference: https://www.kernel.org/doc/Documentation/ABI/testing/dev-kmsg
func ParseMessage(input string, bootTime time.Time) (Message, error) {
	parts := strings.SplitN(input, ";", 2)
	if len(parts) != 2 {
		return Message{}, fmt.Errorf("kernel message should contain a prefix")
	}

	prefix, message := parts[0], parts[1]

	metadata := strings.Split(prefix, ",")
	if len(metadata) < 3 {
		return Message{}, fmt.Errorf("message metdata should have at least 3 parts, got %d", len(metadata))
	}

	syslogPrefix, err := strconv.ParseInt(metadata[0], 10, 64)
	if err != nil {
		return Message{}, fmt.Errorf("error parsing priority: %w", err)
	}

	sequence, err := strconv.ParseInt(metadata[1], 10, 64)
	if err != nil {
		return Message{}, fmt.Errorf("error parsing sequence: %w", err)
	}

	clock, err := strconv.ParseInt(metadata[2], 10, 64)
	if err != nil {
		return Message{}, fmt.Errorf("errors parsing clock from boot: %w", err)
	}

	return Message{
		Priority:       Priority(syslogPrefix & 7),
		Facility:       Facility(syslogPrefix >> 3),
		SequenceNumber: sequence,
		Clock:          clock,
		Timestamp:      bootTime.Add(time.Duration(clock) * time.Microsecond),
		Message:        message,
	}, nil
}
