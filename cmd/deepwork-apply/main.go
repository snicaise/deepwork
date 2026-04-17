// Command deepwork-apply is the privileged half of deepwork.
// It is invoked via NOPASSWD sudo from the user-space deepwork binary.
//
// Two subcommands:
//
//	deepwork-apply apply DOMAIN [DOMAIN ...]   # write block to /etc/hosts
//	deepwork-apply clear                       # remove block from /etc/hosts
//
// Every domain is re-validated here — the sudoers entry grants NOPASSWD, so
// this binary is the security boundary.
package main

import (
	"fmt"
	"os"

	"github.com/sebastien/deepwork/internal/dns"
	"github.com/sebastien/deepwork/internal/domain"
	"github.com/sebastien/deepwork/internal/hosts"
)

const HostsPath = "/etc/hosts"

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "apply":
		if err := doApply(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "deepwork-apply: %v\n", err)
			os.Exit(1)
		}
	case "clear":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "clear takes no arguments")
			os.Exit(2)
		}
		if err := doClear(); err != nil {
			fmt.Fprintf(os.Stderr, "deepwork-apply: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version":
		fmt.Println("deepwork-apply", version)
	default:
		usage()
		os.Exit(2)
	}
}

func doApply(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("apply requires at least one domain")
	}
	domains, err := domain.ParseAll(args)
	if err != nil {
		return err
	}
	ascii := make([]string, 0, len(domains))
	for _, d := range domains {
		ascii = append(ascii, d.ASCII)
	}
	if err := hosts.Apply(HostsPath, ascii); err != nil {
		return err
	}
	if err := dns.Flush(); err != nil {
		return fmt.Errorf("dns flush: %w", err)
	}
	return nil
}

func doClear() error {
	if err := hosts.Clear(HostsPath); err != nil {
		return err
	}
	if err := dns.Flush(); err != nil {
		return fmt.Errorf("dns flush: %w", err)
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `deepwork-apply — privileged /etc/hosts writer

Usage:
  deepwork-apply apply DOMAIN [DOMAIN ...]
  deepwork-apply clear
  deepwork-apply version

This binary is invoked by the deepwork CLI via NOPASSWD sudo and is not
intended for direct use.`)
}
