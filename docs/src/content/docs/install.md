---
title: Installation
description: Install the srs spaced-repetition TUI on your machine.
---

This guide covers the ways to get the `srs` binary onto your machine.

## Requirements

- **Go 1.22 or newer** if you are installing from source with `go install`.
- A terminal that supports 256 colors (most modern terminals qualify).

## Install with `go install`

The quickest way to get `srs` is to install it directly with the Go toolchain:

```sh
go install github.com/jvcorredor/srs-tui/cmd/srs@latest
```

This builds the latest tagged release and places the `srs` binary in
`$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Build from source

To work against the development version, clone the repository and build it
yourself:

```sh
git clone https://github.com/jvcorredor/srs-tui.git
cd srs-tui
go build -o srs ./cmd/srs
```

The resulting `srs` binary is dropped in the current directory. Move it
somewhere on your `PATH` to make it available everywhere.

## Verify the installation

Confirm that `srs` is installed and reachable:

```sh
srs --help
```

If the command prints the available subcommands, you are ready to go.

## Next steps

Head to the [quick-start guide](/srs-tui/quick-start/) to write your first card
and run a review session.
