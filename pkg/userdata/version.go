package userdata

import (
	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

type Version string

func (v Version) Validate() error {
	var result *multierror.Error
	if v == "" {
		result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "version", "", ErrRequiredSection))
	}

	if v != "1" {
		result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "version", v, ErrInvalidVersion))
	}

	return result.ErrorOrNil()
}
