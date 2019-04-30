package userdata

import (
	"errors"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

var (
	ErrInvalidVersion     = errors.New("Invalid config version")
	ErrRequiredSection    = errors.New("Required userdata section")
	ErrUnsupportedCNI     = errors.New("Unsupported CNI driver")
	ErrInvalidTrustdToken = errors.New("Trustd token is required")
)

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

func (o *OSSecurity) Validate() error {
	var result *multierror.Error

	if o.CA == nil {
		result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.os.ca", "", ErrRequiredSection))
	}

	return result.ErrorOrNil()
}

func (i *Init) Validate() error {
	var result *multierror.Error

	switch i.CNI {
	case "calico":
		return result.ErrorOrNil()
	case "flannel":
		return result.ErrorOrNil()
	default:
		result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "services.init.cni", i.CNI, ErrUnsupportedCNI))
	}

	return result.ErrorOrNil()
}

func (t *Trustd) Validate() error {
	var result *multierror.Error

	if t.Token == "" {
		result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "services.trustd.token", t.Token, ErrInvalidTrustdToken))
	}

	return result.ErrorOrNil()
}
