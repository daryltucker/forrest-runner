package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/daryltucker/forest-runner/internal/assets"
	"github.com/daryltucker/forest-runner/internal/output"
	"github.com/spf13/cobra"
)

var functionsCmd = &cobra.Command{
	Use:   "functions",
	Short: "Manage JQ functions for result analysis",
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install specialized JQ functions to ~/.config/vecq/functions/",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}

		targetDir := filepath.Join(home, ".config", "vecq", "functions")
		output.Logger.Info("Installing JQ functions...", "target", targetDir)

		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
		}

		// Read embedded files from internal/assets/functions/
		entries, err := fs.ReadDir(assets.Functions, "functions")
		if err != nil {
			return fmt.Errorf("failed to read embedded functions: %w", err)
		}

		count := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			// Read file content
			content, err := fs.ReadFile(assets.Functions, "functions/"+entry.Name())
			if err != nil {
				output.Logger.Error("Failed to read embedded file", "file", entry.Name(), "error", err)
				continue
			}

			// Write to target
			targetPath := filepath.Join(targetDir, entry.Name())
			if err := os.WriteFile(targetPath, content, 0644); err != nil {
				output.Logger.Error("Failed to write to target", "path", targetPath, "error", err)
				continue
			}

			output.Logger.Info("Installed function", "name", entry.Name())
			count++
		}

		output.Logger.Info("Installation Complete", "total_files", count)
		return nil
	},
}

func init() {
	functionsCmd.AddCommand(installCmd)
	rootCmd.AddCommand(functionsCmd)
}
