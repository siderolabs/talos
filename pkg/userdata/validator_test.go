package userdata

import (
	"testing"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

func TestValidateUserData(t *testing.T) {
	var err error
	data := &UserData{}

	if err = ValidateUserData(data); lookForError(err, ErrRequiredSection) == nil {
		t.Errorf("should have required version section and did not")
	}

	data.Version = "2"
	if err = ValidateUserData(data); lookForError(err, ErrInvalidVersion) == nil {
		t.Errorf("accepted userdata version %q and shouldnt have", data.Version)
	}

	data.Version = "1"
	if err = ValidateUserData(data); err != nil {
		t.Error(err)
	}
}

func TestValidateSecurity(t *testing.T) {
	var err error
	data := &Security{}

	if err = ValidateSecurity(data); lookForError(err, ErrRequiredSection) == nil {
		t.Errorf("should have required security section and did not")
	}

	data.OS = &OSSecurity{}
	data.Kubernetes = &KubernetesSecurity{}
	if err = ValidateSecurity(data); lookForError(err, ErrRequiredSection) != nil {
		t.Error(err)
	}
	/*
		if err = ValidateSecurity(data); lookForError(err, ErrRequiredSection) == nil {
			t.Errorf("should have required version section and did not")
		}

		data.OS.CA = &x509.PEMEncodedCertificateAndKey{}
		if err = ValidateSecurity(data); lookForError(err, ErrRequiredSection) != nil {
			t.Error(err)
		}
	*/
}

func lookForError(err error, target error) error {
	var merr *multierror.Error
	var ok bool

	if merr, ok = err.(*multierror.Error); !ok {
		return err
	}

	for _, er := range merr.Errors {
		if xerrors.Is(er, target) {
			return er
		}
	}

	return nil
}
