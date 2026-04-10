package main

import (
	"encoding/json"
	"fmt"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/spf13/cobra"
)

var configAll bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or modify AJ configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a config value",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.Config)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if configAll || len(args) == 0 {
			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		val, err := config.GetField(cfg, args[0])
		if err != nil {
			return err
		}
		fmt.Println(val)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.Config)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		updated, err := config.SetField(cfg, args[0], args[1])
		if err != nil {
			return err
		}

		if err := paths.EnsureDirs(); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
		if err := config.Save(paths.Config, updated); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("[AJ] Set %s = %s\n", args[0], args[1])
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		if err := paths.EnsureDirs(); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
		if err := config.Save(paths.Config, config.DefaultConfig()); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Println("[AJ] Config reset to defaults")
		return nil
	},
}

func init() {
	configGetCmd.Flags().BoolVar(&configAll, "all", false, "Dump full config")
	configCmd.AddCommand(configGetCmd, configSetCmd, configResetCmd)
	rootCmd.AddCommand(configCmd)
}
