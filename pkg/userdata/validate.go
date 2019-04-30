package userdata

import (
	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

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
