package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the AJ daemon",
}

var ifNotRunning bool
var foreground bool

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the AJ daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		daemon.CleanupStalePID(paths.PID)

		if ifNotRunning && daemon.IsRunning(paths.PID) {
			pid, _ := daemon.ReadPID(paths.PID)
			cfg, err := config.Load(paths.Config)
			if err != nil {
				cfg = config.DefaultConfig()
			}
			// Output context for SessionStart hook
			ctx := map[string]string{
				"additionalContext": fmt.Sprintf(
					"[AJ] Ingestion active. Daemon PID %d. Compile mode: %s.",
					pid, cfg.Compile.TriggerMode),
			}
			data, _ := json.Marshal(ctx)
			fmt.Println(string(data))
			return nil
		}

		if daemon.IsRunning(paths.PID) {
			return fmt.Errorf("daemon already running")
		}

		paths.EnsureDirs()

		cfg, err := config.Load(paths.Config)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		if !foreground {
			// Start as background process
			if err := daemon.StartDaemonProcess(); err != nil {
				return err
			}
			fmt.Println("[AJ] Daemon started in background")
			return nil
		}

		// Foreground mode — run the server directly
		if err := daemon.WritePID(paths.PID); err != nil {
			return fmt.Errorf("writing PID: %w", err)
		}
		defer daemon.RemovePID(paths.PID)

		socketPath := paths.Socket
		if cfg.Daemon.SocketPath != "" {
			socketPath = cfg.Daemon.SocketPath
		}

		srv := daemon.NewServer(socketPath, paths, cfg)
		fmt.Printf("[AJ] Daemon started (PID %d)\n", os.Getpid())
		return srv.Start()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the AJ daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		if !daemon.IsRunning(paths.PID) {
			fmt.Println("[AJ] Daemon is not running")
			return nil
		}

		// Send shutdown signal via socket
		conn, err := net.DialTimeout("unix", paths.Socket, 2*time.Second)
		if err != nil {
			// Can't connect — kill the process
			pid, _ := daemon.ReadPID(paths.PID)
			proc, _ := os.FindProcess(pid)
			if proc != nil {
				_ = proc.Signal(os.Interrupt)
			}
			daemon.RemovePID(paths.PID)
			fmt.Println("[AJ] Daemon stopped (via signal)")
			return nil
		}
		_, _ = conn.Write([]byte("SHUTDOWN\n"))
		conn.Close()

		fmt.Println("[AJ] Daemon stopped")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		if !daemon.IsRunning(paths.PID) {
			fmt.Println("[AJ] Daemon is not running")
			return nil
		}

		pid, _ := daemon.ReadPID(paths.PID)
		fmt.Printf("[AJ] Daemon running (PID %d)\n", pid)
		return nil
	},
}

func init() {
	daemonStartCmd.Flags().BoolVar(&ifNotRunning, "if-not-running", false, "Start only if not already running")
	daemonStartCmd.Flags().BoolVar(&foreground, "foreground", false, "Run in foreground")
	daemonCmd.AddCommand(daemonStartCmd, daemonStopCmd, daemonStatusCmd)
	rootCmd.AddCommand(daemonCmd)
}
