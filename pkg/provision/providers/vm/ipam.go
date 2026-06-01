// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
)

// IPAMRecord describes a single record about a node.
type IPAMRecord struct {
	IP          netip.Addr
	Netmask     byte
	MAC         string
	Hostname    string
	Gateway     netip.Addr
	MTU         int
	Nameservers []netip.Addr

	TFTPServer       string
	IPXEBootFilename string
}

// IPAMDatabase is a mapping from MAC address to records with IPv4/IPv6 flag.
type IPAMDatabase map[string]map[int]IPAMRecord

const dbFile = "ipam.db"

// DumpIPAMRecord appends IPAM record to the database.
func DumpIPAMRecord(statePath string, record IPAMRecord) error {
	f, err := os.OpenFile(filepath.Join(statePath, dbFile), os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	// need atomic write, buffering json
	b, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("error marshaling IPAM record: %w", err)
	}

	_, err = f.Write(append(b, '\n'))

	return err
}

// LoadIPAMRecords loads all the IPAM records indexed by the MAC address.
func LoadIPAMRecords(statePath string) (IPAMDatabase, error) {
	f, err := os.Open(filepath.Join(statePath, dbFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	defer f.Close() //nolint:errcheck

	result := make(IPAMDatabase)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var record IPAMRecord

		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, err
		}

		ipFormat := 4
		if record.IP.Is6() {
			ipFormat = 6
		}

		if result[record.MAC] == nil {
			result[record.MAC] = make(map[int]IPAMRecord)
		}

		result[record.MAC][ipFormat] = record
	}

	return result, scanner.Err()
}
