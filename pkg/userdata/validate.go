package userdata

import (
	"errors"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

var (
	ErrInvalidVersion  = errors.New("Invalid config version")
	ErrRequiredSection = errors.New("Required userdata section")
)

func (v Version) Validate() error {
	var result *multierror.Error
	if v == "" {
		result = multierror.Append(result, xerrors.Errorf("%q: %w", "version", ErrRequiredSection))
	}

	if v != "1" {
		result = multierror.Append(result, xerrors.Errorf("%q: %w", v, ErrInvalidVersion))
	}

	return result.ErrorOrNil()
}

func (o *OSSecurity) Validate() error {
	var result *multierror.Error

	if o.CA == nil {
		result = multierror.Append(result, xerrors.Errorf("%q: %w", "security.os.ca", ErrRequiredSection))
	}

	return result.ErrorOrNil()
}
