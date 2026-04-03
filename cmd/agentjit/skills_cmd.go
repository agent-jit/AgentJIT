package main

import (
	"fmt"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/skills"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage generated skills",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List generated skills with ROI metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		skillsList, err := skills.ListSkills(paths.Skills)
		if err != nil {
			return err
		}

		if len(skillsList) == 0 {
			fmt.Println("[AJ] No skills generated yet. Run 'aj compile' to compile patterns.")
			return nil
		}

		fmt.Printf("%-25s %-8s %-10s %-10s %s\n", "NAME", "SCOPE", "SAVINGS", "FREQ", "TOTAL SAVINGS")
		fmt.Printf("%-25s %-8s %-10s %-10s %s\n", "----", "-----", "-------", "----", "-------------")

		for _, s := range skillsList {
			fmt.Printf("%-25s %-8s %-10d %-10d %d\n",
				s.Name, s.Scope, s.SavingsPerInvocation, s.ObservedFrequency, s.TotalSavings)
		}

		return nil
	},
}

var skillsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a generated skill and deregister it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		if err := skills.RemoveSkill(paths.Skills, args[0]); err != nil {
			return err
		}

		// Also remove symlink from Claude Code skills directory
		claudeSkillsDir, csErr := config.ClaudeSkillsGlobal()
		if csErr == nil {
			skills.UnlinkSkill(claudeSkillsDir, args[0])
		}

		fmt.Printf("[AJ] Removed skill: %s\n", args[0])
		return nil
	},
}

func init() {
	skillsCmd.AddCommand(skillsListCmd, skillsRemoveCmd)
	rootCmd.AddCommand(skillsCmd)
}
