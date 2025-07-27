// ValidateNetworkDevices ensures that an appropriate network.device.interface
// is defined for the given machine type and that IP addresses are compliant.
func ValidateNetworkDevices(cfg *v1alpha1.Config) ([]string, error) {
	var (
		warnings []string
		result   *multierror.Error
	)

	// ...
}

// ValidateTalosVersion checks if the provided version is valid relative to the current version.
// It allows:
// - Current version and any previous version
// - Next minor version within the same major version
func ValidateTalosVersion(requestedVersion, currentVersion string) error {
	// Allow empty version as it defaults to current
	if requestedVersion == "" {
		return nil
	}

	// Parse the requested version
	reqVer, err := semver.ParseTolerant(requestedVersion)
	if err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}

	// Parse the current Talos version
	curVer, err := semver.ParseTolerant(currentVersion)
	if err != nil {
		return fmt.Errorf("failed to parse current Talos version: %w", err)
	}

	// Allow current version and any previous version
	if reqVer.LTE(curVer) {
		return nil
	}

	// For future versions, only allow the next minor version
	nextMinor := semver.Version{
		Major: curVer.Major,
		Minor: curVer.Minor + 1,
		Patch: 0,
	}

	if reqVer.Major == curVer.Major && reqVer.Minor == nextMinor.Minor {
		return nil
	}

	return fmt.Errorf("version %s is too far ahead of current version %s, only current version and next minor version are supported",
		reqVer.String(), curVer.String())
}

// ValidateTalosVersion validates that the provided Talos version is within the supported range.
// It allows:
// - The current version
// - Any previous version
// - One minor version ahead (within the same major version)
func ValidateTalosVersion(requestedVersion string) error {
	// Empty version is valid (uses default)
	if requestedVersion == "" {
		return nil
	}
	
	// Parse the requested version
	if !strings.HasPrefix(requestedVersion, "v") {
		requestedVersion = "v" + requestedVersion
	}
	
	reqVersion, err := semver.NewVersion(requestedVersion)
	if err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}
	
	// Parse the current Talos version
	currentVersion := constants.Version
	if !strings.HasPrefix(currentVersion, "v") {
		currentVersion = "v" + currentVersion
	}
	
	curVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("failed to parse current Talos version: %w", err)
	}
	
	// Allow current version and any previous version
	if reqVersion.LessThanOrEqual(curVersion) {
		return nil
	}
	
	// Allow one minor version ahead (within same major)
	if reqVersion.Major() == curVersion.Major() && 
	   reqVersion.Minor() == curVersion.Minor()+1 {
		return nil
	}
	
	return fmt.Errorf("version %s is not supported; current version is %s, supported versions are current, earlier, or next minor version", 
		reqVersion.Original(), curVersion.Original())
}
