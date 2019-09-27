/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"net"
	"regexp"
	"strconv"

	"github.com/hashicorp/go-multierror"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/xerrors"
)

// ValidHostnamePattern is a pattern which should match valid DNS hostnames according to RFC1123
const ValidHostnamePattern = `^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`

var validHostnameRegex *regexp.Regexp

func init() {
	validHostnameRegex = regexp.MustCompile(ValidHostnamePattern)
}

// Env represents a set of environment variables.
type Env = map[string]string

// Services represents the set of services available to configure.
type Services struct {
	Init    *Init    `yaml:"init"`
	Kubelet *Kubelet `yaml:"kubelet"`
	Kubeadm *Kubeadm `yaml:"kubeadm"`
	Trustd  *Trustd  `yaml:"trustd"`
	Proxyd  *Proxyd  `yaml:"proxyd"`
	OSD     *OSD     `yaml:"osd"`
	CRT     *CRT     `yaml:"crt"`
	NTPd    *NTPd    `yaml:"ntp"`
}

// Validate triggers the specified validation checks to run
func (s *Services) Validate(checks ...ServiceCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(s))
	}

	return result.ErrorOrNil()
}

// ServiceCheck defines the function type for checks
type ServiceCheck func(*Services) error

// CheckServices ensures the minimum necessary services config has been provided
func CheckServices() ServiceCheck {
	return func(s *Services) error {
		var result *multierror.Error

		if s == nil {
			return nil
		}

		if s.Kubeadm == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "services.kubeadm", "", ErrRequiredSection))
		}

		if s.Trustd == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "services.trustd", "", ErrRequiredSection))
		}

		return result.ErrorOrNil()
	}
}

// OSD describes the configuration of the osd service.
type OSD struct {
	CommonServiceOptions `yaml:",inline"`
}

// Proxyd describes the configuration of the proxyd service.
type Proxyd struct {
	CommonServiceOptions `yaml:",inline"`
}

// CRT describes the configuration of the container runtime service.
type CRT struct {
	CommonServiceOptions `yaml:",inline"`
}

// CommonServiceOptions represents the set of options common to all services.
type CommonServiceOptions struct {
	Env Env `yaml:"env,omitempty"`
}

// NTPd describes the configuration of the ntp service.
type NTPd struct {
	CommonServiceOptions `yaml:",inline"`

	Server string `yaml:"server,omitempty"`
}

// Kubelet describes the configuration of the kubelet service.
type Kubelet struct {
	CommonServiceOptions `yaml:",inline"`
	ExtraMounts          []specs.Mount `yaml:"extraMounts"`
}

// Trustd describes the configuration of the Root of Trust (RoT) service. The
// username and password are used by master nodes, and worker nodes. The master
// nodes use them to authenticate clients, while the workers use them to
// authenticate as a client. The endpoints should only be specified in the
// worker user data, and should include all master nodes participating as a RoT.
type Trustd struct {
	CommonServiceOptions `yaml:",inline"`

	Token         string   `yaml:"token"`
	Username      string   `yaml:"username"`
	Password      string   `yaml:"password"`
	Endpoints     []string `yaml:"endpoints,omitempty"`
	CertSANs      []string `yaml:"certSANs,omitempty"`
	BootstrapNode string   `yaml:"bootstrapNode,omitempty"`
}

// TrustdCheck defines the function type for checks
type TrustdCheck func(*Trustd) error

// Validate triggers the specified validation checks to run
func (t *Trustd) Validate(checks ...TrustdCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(t))
	}

	return result.ErrorOrNil()
}

// CheckTrustdAuth ensures that a trustd token has been specified
func CheckTrustdAuth() TrustdCheck {
	return func(t *Trustd) error {
		var result *multierror.Error

		if t.Token == "" && (t.Username == "" || t.Password == "") {
			if t.Token == "" {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "services.trustd.token", t.Token, ErrRequiredSection))
			} else {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "services.trustd.username:password", t.Username+":"+t.Password, ErrRequiredSection))
			}
		}

		return result.ErrorOrNil()
	}
}

// CheckTrustdEndpointsAreValidIPsOrHostnames ensures that the specified trustd endpoints
/// are valid IP addresses or DNS hostnames.
func CheckTrustdEndpointsAreValidIPsOrHostnames() TrustdCheck {
	return func(t *Trustd) error {
		var result *multierror.Error

		for idx, endpoint := range t.Endpoints {
			if ip := net.ParseIP(endpoint); ip != nil {
				continue
			}
			if validHostnameRegex.MatchString(endpoint) {
				continue
			}
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "services.trustd.endpoints["+strconv.Itoa(idx)+"]", endpoint, ErrInvalidAddress))
		}

		return result.ErrorOrNil()
	}
}

// CheckTrustdEndpointsArePresent ensures that tustd endpoints are present.
func CheckTrustdEndpointsArePresent() TrustdCheck {
	return func(t *Trustd) error {
		var result *multierror.Error

		if len(t.Endpoints) == 0 {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "services.trustd.endpoints", t.Endpoints, ErrRequiredSection))
		}

		return result.ErrorOrNil()
	}
}

// Init describes the configuration of the init service.
type Init struct {
	CNI string `yaml:"cni,omitempty"`
}

// InitCheck defines the function type for checks
type InitCheck func(*Init) error

// Validate triggers the specified validation checks to run
func (i *Init) Validate(checks ...InitCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(i))
	}

	return result.ErrorOrNil()
}

// CheckInitCNI ensures that a valid cni driver has been specified
func CheckInitCNI() InitCheck {
	return func(i *Init) error {
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
}
