package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Version information. These can be set at build time using -ldflags.
var (
	Version = "dev"
)

func showHelp() {
	fmt.Println("Usage: infra <action>")
	fmt.Println()
	fmt.Println("Available actions:")
	fmt.Println("  init    - Create default config file")
	fmt.Println("  up      - Start the infrastructure")
	fmt.Println("  start   - Stop running containers")
	fmt.Println("  stop    - Start running containers")
	fmt.Println("  down    - Stop the infrastructure")
	fmt.Println("  pull    - Pull latest images")
	fmt.Println("  backup  - Backup data")
	fmt.Println("  restart - Restart the infrastructure")
	fmt.Println("  version - Show version information")
	fmt.Println("  upgrade - Upgrade to the latest GitHub release")
}

func printVersion() {
	fmt.Println(Version)
}

func relaunchWithSudo(args []string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate infra executable: %w", err)
	}

	cmd := exec.Command("sudo", append([]string{executable}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	isBackup := len(os.Args) >= 2 && os.Args[1] == "backup"
	if isBackup && os.Geteuid() != 0 {
		if err := relaunchWithSudo(os.Args[1:]); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintf(os.Stderr, "Error relaunching backup with sudo: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if !isBackup {
		fmt.Println("Docker Self-Hosted Infra")
		fmt.Printf("Version: ")
		printVersion()
		fmt.Println()
	}

	if len(os.Args) >= 2 && os.Args[1] == "upgrade" {
		if err := upgrade(); err != nil {
			fmt.Fprintf(os.Stderr, "Error upgrading: %v\n", err)
			os.Exit(1)
		}
		return
	}

	configPath := "infra.json"
	cfg, err := ensureConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		showHelp()
		return
	}

	action := os.Args[1]
	switch action {
	case "up":
		fmt.Println("Starting infrastructure...")
		dockerUp()
	case "start":
		fmt.Println("Starting infrastructure...")
		dockerStart(false)
	case "down":
		fmt.Println("Stopping infrastructure...")
		dockerDown()
	case "stop":
		fmt.Println("Stopping infrastructure...")
		dockerStop(false)
	case "pull":
		fmt.Println("Pulling latest images...")
		dockerPull()
	case "backup":
		dockerStop(true)
		err := backupData(cfg.ArchiveName, cfg.Exclude)
		dockerStart(true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error backing up data: %v\n", err)
			os.Exit(1)
		}
	case "restart":
		fmt.Println("Restarting infrastructure...")
		dockerRestart()
	case "version":
		printVersion()
	case "init":
		initConfig(configPath)
		return
	default:
		fmt.Printf("Unknown action: %s\n\n", action)
		showHelp()
	}
}
