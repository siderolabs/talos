package userdata

import "errors"

var (
	ErrInvalidVersion     = errors.New("invalid config version")
	ErrRequiredSection    = errors.New("required userdata section")
	ErrUnsupportedCNI     = errors.New("unsupported CNI driver")
	ErrInvalidTrustdToken = errors.New("trustd token is required")
	ErrInvalidCert        = errors.New("Certificate is invalid")
	ErrInvalidCertType    = errors.New("Certificate type is invalid")
)
