package userdata

import "errors"

var (
	// General
	ErrRequiredSection = errors.New("required userdata section")
	ErrInvalidVersion  = errors.New("invalid config version")

	// Security
	ErrInvalidCert     = errors.New("Certificate is invalid")
	ErrInvalidCertType = errors.New("Certificate type is invalid")

	// Services
	ErrUnsupportedCNI     = errors.New("unsupported CNI driver")
	ErrInvalidTrustdToken = errors.New("trustd token is invalid")
)
