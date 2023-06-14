// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sdboot

import (
	"errors"

	"github.com/ecks/uefi/efi/efiguid"
	"github.com/ecks/uefi/efi/efivario"
	"golang.org/x/text/encoding/unicode"
)

// SystemdBootGUIDString is the GUID of the SystemdBoot EFI variables.
const SystemdBootGUIDString = "4a67b082-0a4c-41cf-b6c7-440b29bb8c4f"

// SystemdBootGUID is the GUID of the SystemdBoot EFI variables.
var SystemdBootGUID = efiguid.MustFromString(SystemdBootGUIDString)

// Variable names.
const (
	LoaderEntryDefaultName  = "LoaderEntryDefault"
	LoaderEntrySelectedName = "LoaderEntrySelected"
	LoaderConfigTimeoutName = "LoaderConfigTimeout"
)

// ReadVariable reads a SystemdBoot EFI variable.
func ReadVariable(c efivario.Context, name string) (string, error) {
	_, data, err := efivario.ReadAll(c, name, SystemdBootGUID)
	if err != nil {
		if errors.Is(err, efivario.ErrNotFound) {
			return "", nil
		}

		return "", err
	}

	out := make([]byte, len(data))

	decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()

	n, _, err := decoder.Transform(out, data, true)
	if err != nil {
		return "", err
	}

	if n > 0 && out[n-1] == 0 {
		n--
	}

	return string(out[:n]), nil
}

// WriteVariable reads a SystemdBoot EFI variable.
func WriteVariable(c efivario.Context, name, value string) error {
	out := make([]byte, (len(value)+1)*2)

	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()

	n, _, err := encoder.Transform(out, []byte(value), true)
	if err != nil {
		return err
	}

	out = append(out[:n], 0, 0)

	return c.Set(name, SystemdBootGUID, efivario.BootServiceAccess|efivario.RuntimeAccess|efivario.NonVolatile, out)
}
