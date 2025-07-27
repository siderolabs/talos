// genCmd represents the gen command.
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate CAs, certificates, and private keys",
	Long:  ``,
}

func init() {
	addCommand(rootCmd, genCmd)
}

// validateTalosVersion checks if the provided version is valid relative to the current version
func validateTalosVersion(version string) error {
	return machinery.ValidateTalosVersion(version, constants.Version)
}
