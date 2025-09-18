package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func sshConfigPath() string {
	if path := os.Getenv("SSH_CONFIG"); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot get home dir: %v", err)
	}
	return filepath.Join(home, ".ssh", "config")
}

func listHosts(config string) ([]string, error) {
	f, err := os.Open(config)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hosts := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 1 && strings.ToLower(fields[0]) == "host" {
			for _, h := range fields[1:] {
				if strings.ContainsAny(h, "*?!") {
					continue
				}
				hosts[h] = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(hosts))
	for h := range hosts {
		result = append(result, h)
	}
	sort.Strings(result)
	return result, nil
}

func pickHost(hosts []string) (string, error) {
	if len(hosts) == 0 {
		return "", errors.New("no hosts found")
	}

	if _, err := exec.LookPath("fzf"); err == nil {
		cmd := exec.Command("fzf", "--prompt=ssh → ", "--height=40%", "--reverse", "--border")
		cmd.Stdin = strings.NewReader(strings.Join(hosts, "\n"))
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	fmt.Println("Select a host:")
	for i, h := range hosts {
		fmt.Printf("%d) %s\n", i+1, h)
	}
	fmt.Print("> ")

	var choice int
	_, err := fmt.Scan(&choice)
	if err != nil || choice < 1 || choice > len(hosts) {
		return "", errors.New("invalid choice")
	}
	return hosts[choice-1], nil
}

func usage() {
	prog := filepath.Base(os.Args[0])
	fmt.Printf(`Usage: %s [--sftp] [--print] [-- command args...]
(no args) → pick a host and ssh into it
--sftp   → pick a host and open sftp
--print  → just print chosen host
Examples:
  %s
  %s --sftp
  %s -- -L 8080:localhost:80
`, prog, prog, prog, prog)
}

func main() {
	config := sshConfigPath()
	if _, err := os.Stat(config); err != nil {
		fmt.Fprintf(os.Stderr, "No readable SSH config at %s\n", config)
		os.Exit(1)
	}

	mode := "ssh"
	printOnly := false
	var passArgs []string

	args := os.Args[1:]
	for len(args) > 0 {
		switch args[0] {
		case "--sftp":
			mode = "sftp"
			args = args[1:]
		case "--print":
			printOnly = true
			args = args[1:]
		case "-h", "--help":
			usage()
			return
		case "--":
			passArgs = args[1:]
			args = nil
		default:
			passArgs = append(passArgs, args[0])
			args = args[1:]
		}
	}

	hosts, err := listHosts(config)
	if err != nil {
		log.Fatal(err)
	}
	host, err := pickHost(hosts)
	if err != nil || host == "" {
		fmt.Fprintln(os.Stderr, "No host selected.")
		os.Exit(1)
	}

	if printOnly {
		fmt.Println(host)
		return
	}

	var cmd *exec.Cmd
	if mode == "sftp" {
		cmd = exec.Command("sftp", host)
	} else {
		cmd = exec.Command("ssh", append([]string{host}, passArgs...)...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		os.Exit(cmd.ProcessState.ExitCode())
	}
}
