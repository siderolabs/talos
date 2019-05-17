/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package token

import (
	"github.com/google/uuid"
)

const (
	// BootstrapTTL is the maximum age a token is allowed to be.
	BootstrapTTL uuid.Time = 3600
)

// Token represents a token.
type Token struct {
	uuid uuid.UUID
}

// NewToken initializes and returns a new token.
func NewToken() (*Token, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	t := &Token{
		uuid: uuid,
	}

	return t, nil
}

// FromString returns a token parsed from a string.
func FromString(input string) (*Token, error) {
	uuid, err := uuid.Parse(input)
	if err != nil {
		return nil, err
	}

	t := &Token{
		uuid: uuid,
	}

	return t, nil
}

// Expired checks if the token has expired.
func (t *Token) Expired() bool {
	t1 := t.uuid.Time()

	t2, _, err := uuid.GetTime()
	if err != nil {
		return false
	}

	return t2-t1 < BootstrapTTL
}

func (t *Token) String() string {
	return t.uuid.String()
}

// UnmarshalYAML implements the unmarshaller interface so we can
// represent a UUID v1 token as a string.
func (t *Token) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stoken string

	if err := unmarshal(&stoken); err != nil {
		return err
	}

	token, err := FromString(stoken)
	if err != nil {
		return err
	}

	*t = *token

	return nil
}

// MarshalYAML implements the marshaller interface so we can
// represent a UUID v1 token as a string.
func (t *Token) MarshalYAML() (interface{}, error) {
	return t.uuid.String(), nil
}
