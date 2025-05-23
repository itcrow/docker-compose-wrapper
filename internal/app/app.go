package app

// Execute runs the root command
func Execute() error {
	cmd := NewRootCommand()
	return cmd.Execute()
}
