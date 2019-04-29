package userdata

import (
	"errors"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

var (
	ErrInvalidVersion  = errors.New("Invalid config version")
	ErrRequiredSection = errors.New("Required userdata section")

	ErrRequiredSectionMissingSecurityOSCA   = errors.New("")
	ErrRequiredSectionMissingSecurityKubeCA = errors.New("")

	ErrInvalidCertificate                = errors.New("")
	ErrNotBase64Encoded                  = errors.New("")
	ErrSecurityOSCACrtNotBase64Encoded   = errors.New("")
	ErrSecurityOSCAKeyNotBase64Encoded   = errors.New("")
	ErrSecurityKubeCACrtNotBase64Encoded = errors.New("")
	ErrSecurityKubeCAKeyNotBase64Encoded = errors.New("")
)

// TODO Maybe look at top level Validate function that
// does switch type against the various userdata bits
// and calls the appropriate function

func ValidateUserData(data *UserData) error {
	var result *multierror.Error

	if data.Version == "" {
		result = multierror.Append(result, xerrors.Errorf("%q: %w", "version", ErrRequiredSection))
	}

	if data.Version != "1" {
		result = multierror.Append(result, xerrors.Errorf("%q: %w", data.Version, ErrInvalidVersion))
	}

	return result.ErrorOrNil()
}

func ValidateSecurity(data *Security) error {
	var result *multierror.Error
	if data == nil {
		result = multierror.Append(result, xerrors.Errorf("%q: %w", "security", ErrRequiredSection))
	}

	if data.OS == nil {
		result = multierror.Append(result, xerrors.Errorf("%q: %w", "security.os", ErrRequiredSection))
	} else {
		result = multierror.Append(result, ValidateOSSecurity(data.OS))
	}
	if data.Kubernetes == nil {
		result = multierror.Append(result, xerrors.Errorf("%q: %w", "security.kubernetes", ErrRequiredSection))
	}

	/*
		var err error
		if err = ValidateOSSecurity(data.OS); err != nil {
			// Both of these result + error are multierror.Error
			// result = []error{}
			// err = []error{}
			result = multierror.Append(result, err)
		}
	*/

	return result.ErrorOrNil()

}

func ValidateOSSecurity(data *OSSecurity) error {
	var result *multierror.Error

	if data.CA == nil {
		result = multierror.Append(result, xerrors.Errorf("%q: %w", "security.os.ca", ErrRequiredSection))
	}

	/*
		if data.CA.Crt == nil {
		}
		if data.CA.Key == nil {
		}

		/*
			if _, err = base64.StdEncoding.DecodeString(data.CA.Crt); err != nil {
				err = multierror.Append(err, ErrSecurityOSCACrtNotBase64Encoded)
			}
			if _, err = base64.StdEncoding.DecodeString(data.CA.Key); err != nil {
				err = multierror.Append(err, ErrSecurityOSCAKeyNotBase64Encoded)
			}
	*/

	return result.ErrorOrNil()
}

/*
func ValidateKubernetesSecurity(data *KubernetesSecurity) (err error) {
	if data.CA == nil {
		err = multierror.Append(err, ErrRequiredSectionMissingSecurityKubeCA)
	}
	if data.CA.Crt == nil {
	}
	if data.CA.Key == nil {
	}

	if _, err = base64.StdEncoding.DecodeString(data.CA.Crt); err != nil {
		err = multierror.Append(err, ErrSecurityKubeCACrtNotBase64Encoded)
	}
	if _, err = base64.StdEncoding.DecodeString(data.CA.Key); err != nil {
		err = multierror.Append(err, ErrSecurityKubeCAKeyNotBase64Encoded)
	}
}
func ValidateNetworking(data *Networking) (err error) {
	if data.OS == nil {
		return
	}

	err = multierror.Append(err, ValidateOSNet(data.OS))
}

func ValidateOSNet(data *OSNet) (err error) {
	if len(data.Devices) == 0 {
		return
	}

	for _, device := range data.Devices {
		err = multierror.Append(err, ValidateDevice(device))
	}
}

func ValidateDevice(data *Device) (err error) {
	if data.Interface == "" {
		err = multierror.Append(err, ErrInvalidNetworkDevice)
	}

	if !data.DHCP {
		if _, _, err = net.ParseCIDR(data.CIDR); err != nil {
			err = multierror.Append(err, ErrInvalidCIDR)
		}
	}

	if data.CIDR == "" && !data.DHCP {
		err = multierror.Append(err, ErrInvalidAddressing)
	}

	// TODO Add this back once we get bonding sorted
	// err = multierror.Append(err, ValidateBond(data.Bond))
	err = multierror.Append(err, ValidateRoutes(data.Routes))
}

func ValidateBond(data *Bond) (err error) {
	if data == nil {
		return
	}
}

func ValidateRoute(data *Route) (err error) {
	if len(data) == 0 {
		return
	}

	if _, _, err = net.ParseCIDR(data.Network); err != nil {
	}
	if ip := net.ParseIP(data.Gateway); ip == nil {
	}
}

func ValidateInstall(data *Install) (err error) {
	if data == nil {
		return
	}

	err = multierror.Append(err, ValidateBootDevice(data.Boot))
	err = multierror.Append(err, ValidateRootDevice(data.Root))
	err = multierror.Append(err, ValidateInstallDevice(data.Data))
	err = multierror.Append(err, ValidateExtraDevice(data.ExtraDevices))
}
func ValidateBootDevice(data *BootDevice) (err error) {
	if data == nil {
		return
	}

	if data.Device == "" {
	}
}
func ValidateRootDevice(data *RootDevice) (err error) {
	if data == nil {
		return
	}

	if data.Device == "" {
	}
}
func ValidateInstallDevice(data *InstallDevice) (err error) {
	if data == nil {
		return
	}

	if data.Device == "" {
	}
}
func ValidateExtraDevice(data *ExtraDevice) (err error) {
	if len(data) == 0 {
		return
	}

	if data.Device == "" {
	}
	if len(data.Partitions) == 0 {
	}

	err = multierror.Append(err, ValidateExtraDevicePartition(data.Partitions))
}
func ValidateExtraDevicePartition(data *ExtraDevicePartition) (err error) {
	// Fail here because device was specified but no definition of how to act
	if len(data) == 0 {
	}

	if data.Size == 0 {
	}

	if data.MountPoint == "" {
	}

}

func ValidateServices(data *Services) (err error) {
	if data.Init == nil {
		// Required
	} else {
		err = multierror.Append(err, ValidateInit(data.Init))
	}

	if data.Kubelet != nil {
		err = multierror.Append(err, ValidateKubelet(data.Kubelet))
	}

	if data.Kubeadm == nil {
		// Required
	} else {
		err = multierror.Append(err, ValidateKubeadm(data.Kubeadm))
	}

	if data.Trustd == nil {
		// Required
	} else {
		err = multierror.Append(err, ValidateTrustd(data.Trustd))
	}

	if data.Proxyd != nil {
		err = multierror.Append(err, ValidateProxyd(data.Proxyd))
	}

	if data.OSD != nil {
		err = multierror.Append(err, ValidateOSD(data.OSD))
	}

	if data.CRT != nil {
		err = multierror.Append(err, ValidateCRT(data.CRT))
	}

	if data.NTPd != nil {
		err = multierror.Append(err, ValidateNTPd(data.NTPd))
	}
}

func ValidateInit(data *Init) (err error) {
	switch data.CNI {
	case "calico":
		return
	case "flannel":
		return
	default:
		err = multierror.Append(err, "")
	}
}
func ValidateKubelet(data *Kubelet) (err error) {
	// Should ExtraMounts be omitempty?
	return
}
func ValidateKubeadm(data *Kubeadm) (err error) {
	// Yeah this is gonna be a pain :)
	return
}
func ValidateTrustd(data *Trustd) (err error) {
	return
}
func ValidateOSD(data *OSD) (err error) {
	return
}
func ValidateProxyd(data *Proxyd) (err error) {
	return
}
func ValidateCRT(data *CRT) (err error) {
	return
}
func ValidateNTPd(data *NTPd) (err error) {
	return
}
*/
