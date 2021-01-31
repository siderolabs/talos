// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

// StaticKeyHandler just handles the static key value all the time.
type StaticKeyHandler struct {
	key []byte
}

// NewStaticKeyHandler creates new EphemeralKeyHandler.
func NewStaticKeyHandler(key []byte) (*StaticKeyHandler, error) {
	return &StaticKeyHandler{
		key: key,
	}, nil
}

// GetKey implements KeyHandler interface.
func (h *StaticKeyHandler) GetKey(options ...KeyOption) ([]byte, error) {
	return h.key, nil
}
