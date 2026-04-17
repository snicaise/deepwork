package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "install":
		err = cmdInstall()
	case "uninstall":
		err = cmdUninstall()
	case "start":
		err = cmdStart()
	case "stop":
		err = cmdStop()
	case "status":
		err = cmdStatus()
	case "now":
		err = cmdNow(args)
	case "edit":
		err = cmdEdit()
	case "tick":
		err = cmdTick()
	case "doctor":
		err = cmdDoctor()
	case "version", "--version", "-v":
		fmt.Println("deepwork", version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deepwork:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `deepwork — configurable website blocker

Usage: deepwork <command> [args...]

Commands:
  install       Setup deepwork and install the LaunchAgent (needs sudo)
  uninstall     Remove deepwork completely (needs sudo)
  start         Enable scheduled blocking
  stop          Disable scheduled blocking
  status        Show current state
  now <dur>     Block for a fixed duration (e.g. 25m, 2h, 1h30m)
  edit          Open config in $EDITOR
  doctor        Diagnose DoH and other bypass risks
  version       Print version`)
}
