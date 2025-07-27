// genSecretsCmd represents the gen secrets command.
var genSecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Generate secrets for Talos cluster",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate Talos version
		if err := config.ValidateTalosVersion(genSecretsCmdFlags.TalosVersion); err != nil {
			return fmt.Errorf("invalid --talos-version: %w", err)
		}
		
		// Validate Talos version
		if err := validateTalosVersion(genSecretsCmdFlags.TalosVersion); err != nil {
			return err
		}
		
		inputs := generate.NewInput("", "", generate.WithSecretsBundle(genSecretsCmdFlags.OutputPath))
		inputs.TalosVersion = genSecretsCmdFlags.TalosVersion

		if err := inputs.ApplyVersionContract(); err != nil {
			return err
		}

		secrets, err := generate.NewSecrets()
		if err != nil {
			return fmt.Errorf("failed to generate secrets: %w", err)
		}

		return inputs.Write(secrets, nil, nil, nil, nil)
	},
}
