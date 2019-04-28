package userdata

import (
	"encoding/base64"
	"errors"
	"net"

	"github.com/hashicorp/go-multierror"
)

var (
	ErrInvalidVersion = errors.New("Invalid config version")

	ErrRequiredSectionMissingSecurity       = errors.New("Required section 'security' missing")
	ErrRequiredSectionMissingSecurityOS     = errors.New("Required section 'security.os' missing")
	ErrRequiredSectionMissingSecurityKube   = errors.New("Required section 'security.kubernetes' missing")
	ErrRequiredSectionMissingSecurityOSCA   = errors.New("")
	ErrRequiredSectionMissingSecurityKubeCA = errors.New("")

	ErrInvalidCertificate                = errors.New("")
	ErrNotBase64Encoded                  = errors.New("")
	ErrSecurityOSCACrtNotBase64Encoded   = errors.New("")
	ErrSecurityOSCAKeyNotBase64Encoded   = errors.New("")
	ErrSecurityKubeCACrtNotBase64Encoded = errors.New("")
	ErrSecurityKubeCAKeyNotBase64Encoded = errors.New("")
)

func ValidateUserData(data *UserData) (err error) {
	if data.Version != "1" {
		err = multierror.Append(err, ErrInvalidVersion)
	}
}

func ValidateSecurity(data *Security) (err error) {
	if data == nil {
		err = multierror.Append(err, ErrRequiredSectionMissingSecurity)
	}
	if data.OS == nil {
		err = multierror.Append(err, ErrRequiredSectionMissingSecurityOS)
	}
	if data.Kubernetes == nil {
		err = multierror.Append(err, ErrRequiredSectionMissingSecurityKube)
	}
}

func ValidateOSSecurity(data *OSSecurity) (err error) {
	if data.CA == nil {
		err = multierror.Append(err, ErrRequiredSectionMissingSecurityOSCA)
	}
	if data.CA.Crt == nil {
	}
	if data.CA.Key == nil {
	}

	if _, err = base64.StdEncoding.DecodeString(data.CA.Crt); err != nil {
		err = multierror.Append(err, ErrSecurityOSCACrtNotBase64Encoded)
	}
	if _, err = base64.StdEncoding.DecodeString(data.CA.Key); err != nil {
		err = multierror.Append(err, ErrSecurityOSCAKeyNotBase64Encoded)
	}
}
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
