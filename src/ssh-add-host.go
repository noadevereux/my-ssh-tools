package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"strconv"
)

var (
	force     bool
	alias     string
	hostname  string
	username  string
	port      string
	idfile    string
	proxyjump string
	addKnown  string
)

func usage() {
	prog := filepath.Base(os.Args[0])
	fmt.Printf(`Usage: %s [-f] [-a alias] [-h hostname] [-u user] [-p port] [-i identityfile] [-P proxyjump] [--add-known-hosts yes/no]
Prompts for any missing fields.

Options:
  -f                 Overwrite existing Host alias if it exists
  -a alias           Host alias (e.g., web-prod)
  -h hostname        HostName (IP or DNS)
  -u user            SSH user (e.g., ubuntu)
  -p port            Port (default: 22)
  -i identityfile    Path to private key (e.g., ~/.ssh/id_ed25519)
  -P proxyjump       ProxyJump (e.g., bastion)
  --add-known-hosts  yes|no (default: yes) – run ssh-keyscan to pre-populate known_hosts
`, prog)
}

func prompt(current *string, msg, def string) {
	if *current != "" {
		return
	}
	r := bufio.NewReader(os.Stdin)
	if def != "" {
		fmt.Printf("%s [%s]: ", msg, def)
	} else {
		fmt.Printf("%s: ", msg)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" && def != "" {
		line = def
	}
	*current = line
}

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

func removeExistingAlias(config, alias string) error {
	data, err := os.ReadFile(config)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var out []string
	skip := false
	hostRe := regexp.MustCompile(`(?i)^host\\s+`)
	for _, line := range lines {
		if hostRe.MatchString(line) {
			fields := strings.Fields(line)
			hit := false
			for _, f := range fields[1:] {
				if f == alias {
					hit = true
				}
			}
			skip = hit
		}
		if !skip {
			out = append(out, line)
		}
	}

	backup := fmt.Sprintf("%s.%s.bak", config, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, data, 0600); err != nil {
		return err
	}

	return os.WriteFile(config, []byte(strings.Join(out, "\n")), 0600)
}

func appendBlock(config string) error {
	f, err := os.OpenFile(config, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Host %s\n", alias)
	fmt.Fprintf(w, "    HostName %s\n", hostname)
	fmt.Fprintf(w, "    User %s\n", username)
	if port != "" && port != "22" {
		fmt.Fprintf(w, "    Port %s\n", port)
	}
	if idfile != "" {
		fmt.Fprintf(w, "    IdentityFile %s\n", idfile)
	}
	if proxyjump != "" {
		fmt.Fprintf(w, "    ProxyJump %s\n", proxyjump)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}

func addKnownHosts(hostname, port string) {
	args := []string{"-T", "5"}
	if port != "" && port != "22" {
		args = append(args, "-p", port)
	}
	args = append(args, hostname)

	cmd := exec.Command("ssh-keyscan", args...)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	home, _ := os.UserHomeDir()
	known := filepath.Join(home, ".ssh", "known_hosts")
	f, err := os.OpenFile(known, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(out)

	// deduplicate
	data, err := os.ReadFile(known)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	uniq := map[string]bool{}
	var outLines []string
	for _, l := range lines {
		if l == "" {
			continue
		}
		if !uniq[l] {
			uniq[l] = true
			outLines = append(outLines, l)
		}
	}
	sort.Strings(outLines)
	os.WriteFile(known, []byte(strings.Join(outLines, "\n")), 0600)
}

func main() {
	flag.BoolVar(&force, "f", false, "force overwrite")
	flag.StringVar(&alias, "a", "", "alias")
	flag.StringVar(&hostname, "h", "", "hostname")
	flag.StringVar(&username, "u", "", "user")
	flag.StringVar(&port, "p", "", "port")
	flag.StringVar(&idfile, "i", "", "identity file")
	flag.StringVar(&proxyjump, "P", "", "proxyjump")
	flag.StringVar(&addKnown, "add-known-hosts", "", "add known hosts")
	flag.Usage = usage
	flag.Parse()

	prompt(&alias, "Host alias (unique, no spaces)", "")
	prompt(&hostname, "HostName (DNS or IP)", "")
	prompt(&username, "User", os.Getenv("USER"))
	prompt(&port, "Port", "22")
	prompt(&idfile, "IdentityFile path (optional, blank to skip)", "")
	prompt(&proxyjump, "ProxyJump (optional, blank to skip)", "")
	prompt(&addKnown, "Add to known_hosts via ssh-keyscan? yes/no", addKnown)

	if alias == "" || hostname == "" || username == "" || port == "" {
		log.Fatal("missing required fields")
	}

	port = strings.TrimSpace(port)
	if port == "" {
		log.Fatal("port must not be empty")
	}

	pnum, err := strconv.Atoi(port)
	if err != nil || pnum <= 0 || pnum > 65535 {
		log.Fatal("port must be a number between 1 and 65535")
	}

	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	os.MkdirAll(sshDir, 0700)
	config := sshConfigPath()
	if _, err := os.Stat(config); errors.Is(err, os.ErrNotExist) {
		os.WriteFile(config, []byte{}, 0600)
	}

	exists := false
	data, _ := os.ReadFile(config)
	if regexp.MustCompile(fmt.Sprintf(`(?i)^host\\s+%s(\\s|$)`, regexp.QuoteMeta(alias))).Match(data) {
		exists = true
	}

	if exists {
		if !force {
			fmt.Fprintf(os.Stderr, "Host \"%s\" already exists in %s. Use -f to overwrite.\n", alias, config)
			os.Exit(2)
		}
		if err := removeExistingAlias(config, alias); err != nil {
			log.Fatal(err)
		}
	}

	if err := appendBlock(config); err != nil {
		log.Fatal(err)
	}

	if strings.ToLower(addKnown) == "yes" {
		addKnownHosts(hostname, port)
	}

	fmt.Printf("Added Host \"%s\" to %s.\n", alias, config)
}