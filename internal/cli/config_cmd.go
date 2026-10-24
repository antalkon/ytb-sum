package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/antalkon/ytb-sum/internal/config"
)

func newConfigCmd(fl *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the ytb-sum configuration file",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create a commented config file (mode 600)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if errors.Is(err, fs.ErrExist) {
				return fmt.Errorf("%s already exists", path)
			}
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := f.WriteString(config.Template); err != nil {
				return err
			}
			fmt.Println("Created", path)
			fmt.Println("Store the key outside the file, e.g. in the macOS Keychain:")
			fmt.Printf("  security add-generic-password -a \"$USER\" -s %s -w\n", config.KeychainService)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print the resolved configuration (API key masked)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := resolveConfig(cmd.Root(), *fl)
			if err != nil {
				return err
			}
			path, _ := config.Path()
			src := cfg.APIKeySource
			if src == "" {
				src = "none"
			}
			rows := [][2]string{
				{"config file", path},
				{"api key", config.MaskKey(cfg.APIKey) + "  (" + src + ")"},
				{"base url", cfg.BaseURL},
				{"quality", cfg.Quality},
				{"model", cfg.ResolvedModel()},
				{"fallback model", cfg.FallbackModel},
				{"audio model", cfg.AudioModel},
				{"lang", cfg.Lang},
				{"length", cfg.Length},
				{"sub lang", orDash(cfg.SubLang)},
				{"output dir", orDash(cfg.OutputDir)},
				{"audio fallback", fmt.Sprint(cfg.AudioFallback)},
				{"timeout", cfg.Timeout.String()},
			}
			for _, r := range rows {
				fmt.Printf("%-15s %s\n", r[0], r[1])
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the config file location",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			p, err := config.Path()
			if err == nil {
				fmt.Println(p)
			}
			return err
		},
	})
	return cmd
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
