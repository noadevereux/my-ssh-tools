# my-ssh-tools

A set of simple command-line utilities for convenient management and connection to SSH hosts.

## Features

- **ssh-menu**: Interactive SSH host picker and launcher.
  - Lists all hosts from your SSH config.
  - Uses [fzf](https://github.com/junegunn/fzf) for fast search if installed.
  - Supports direct SSH, SFTP, or passing additional arguments.
  - Can simply print the selected host.

- **ssh-add-host**: Easy addition of SSH hosts to your config.
  - Prompts for all required fields (alias, hostname, user, port, etc.).
  - Allows overwriting an existing alias.
  - Can pre-populate `known_hosts` using `ssh-keyscan`.
  - Creates a backup of your config before changes.

## Installation

Soon

## Usage

### ssh-menu

```sh
ssh-menu                # Pick a host and connect via SSH
ssh-menu --sftp         # Pick a host and open SFTP
ssh-menu --print        # Only print the selected host
ssh-menu -- -L 8080:localhost:80  # Pass additional SSH arguments
```

### ssh-add-host

```sh
ssh-add-host            # Interactive mode with prompts for all fields
ssh-add-host -a web-prod -h 1.2.3.4 -u ubuntu -p 22 --add-known-hosts yes
ssh-add-host -f ...     # Overwrite an existing alias
```

## SSH Config

Both utilities use the default SSH config: `~/.ssh/config`. You can override the path using the `SSH_CONFIG` environment variable.
