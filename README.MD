# Docker Self-Hosted Infra

This project is the successor of [this project](https://github.com/sirber/infra).
It is written in GoLang instead of Makefile. Ths biggest advantage is that in order 
to use the infra, you won't have to merge two git repositories, one for the infra, 
the other for `services/`.

# Installation

Install the latest Linux/amd64 release on Debian, Ubuntu, Fedora, or another
compatible Linux distribution:

```sh
wget -qO- https://raw.githubusercontent.com/sirber/docker-infra/main/install.sh | bash
```

The installer requires Bash and wget or curl. It installs `infra` to
`~/.local/bin` without sudo. If that directory is not on `PATH`, it offers to
add it to your Bash, Zsh, or sh configuration when a terminal is available.
Open a new shell or run `export PATH="$HOME/.local/bin:$PATH"` to use it in
the current shell. For other shells or non-interactive installs, configure
`PATH` manually.

Alternatively, install the automatically compiled binary manually:

- Download [latest release](https://github.com/sirber/docker-infra/releases/latest)
  - example: `wget -O infra https://github.com/sirber/docker-infra/releases/latest/download/infra`
- Move the downloaded binary `infra` to a folder in path
- Make it executable: `chmod a+x infra`

# Usage

In a peculiar folder, like `~/infra`, init your infra by running `infra init`. 
A `infra.jon` config file will be created. Then, you can create folders and sub-folders
for your services. 

At minimum, a service must have a `docker-compose.yml` and be enabled by creating
an empty file `enabled`. 

Then, you are ready to start the infra! With `infra up`.

## Upgrade

Run `infra upgrade` to check the latest GitHub release and display the installed
and available versions. If a newer version is available, confirm with `y` or
`yes` to download and replace the installed executable, then exit. Other
answers cancel.
Development builds (`dev`) can also be replaced with the latest release.

Self-upgrade supports Linux/amd64, does not require `infra.json`, and requires
write permission to the installation directory. The executable's permissions are
preserved, and failed downloads leave the existing installation untouched.

## Best Practices

- Use [bind mount](https://docs.docker.com/engine/storage/bind-mounts/), to be able to backup your data
- Set [restart policy](https://www.baeldung.com/ops/docker-compose-restart-policies) to "always"
- Down your service before deleting it, or else it will run forever like a ghost!

## Backup

You can do a full cold backup by running `infra backup`; the command will request
root access through `sudo` when needed. The backup will keep file permission for
the bind mounts. The archive file name is configured with `archive_name` in `infra.json`. You can
also configure an `exclude` array of glob patterns, for example:

```json
{
  "archive_name": "backup.tar.lz4",
  "exclude": ["*.zip", "Cache/"]
}
```

The archive extension selects compression: `.gz` uses gzip and `.lz4` uses
LZ4. New configurations default to `backup.tar.lz4`.

Patterns without a `/` match path components at any depth; patterns with a `/`
match the path relative to the infrastructure directory. A trailing `/` limits
a pattern to directories, which are skipped along with their contents. The
archive itself is always excluded. After any `sudo` prompt, backup shows the
standard version header and announces when it stops services, creates the
archive, and starts services again. Docker Compose output is streamed as it is
for other commands. While creating the archive, backup prints one dot for each
included directory containing `docker-compose.yml` or `compose.yml`, all on one
line (`.....` for five matching directories).

To restore, just extract the archive somewhere else (as root) and up it!

# Development

TODO