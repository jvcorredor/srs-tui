# 1. Auto-slugify card and deck names

Date: 2026-05-17

## Status

Accepted

## Context

Decks are directories and cards are `.md` files on disk. Users supply deck and
card names as free text on the command line and in the TUI (`srs new`,
`srs deck create`, the `N` keybinding). Free text contains spaces, punctuation,
mixed case, and unicode — none of which makes a good filesystem identifier.
Without normalisation we would either reject perfectly reasonable names or
create files and directories whose names are awkward to type, inconsistent
across platforms, and ambiguous to match later.

We also need a single, predictable rule so that the same human-readable name
always resolves to the same on-disk identifier, regardless of which entry point
created it.

## Decision

Every human-supplied card and deck name is passed through a shared
`internal/slug.Slugify` helper before it touches the filesystem. Slugify:

- lowercases the input,
- replaces every run of non-`[a-z0-9]` runes with a single hyphen,
- strips leading and trailing hyphens.

The slug is the canonical on-disk identifier. `My Spanish Verbs!` becomes
`my-spanish-verbs`; `To Be (ser)` becomes `to-be-ser`. Input with no
alphanumeric characters slugifies to the empty string and is rejected as a
usage error.

`Slugify` lives in its own package so the CLI and the TUI share exactly one
implementation and cannot drift.

## Consequences

- Card and deck names on disk are consistent, portable, and easy to type.
- A name and its slug are not the same string; commands that look up an
  existing deck must slugify the argument before matching.
- Two distinct display names can collapse to the same slug (`C#` and `C++`
  both slugify to `c`); the second attempt to create that identifier fails as
  "already exists", which is acceptable for a single-user local tool.
- All-punctuation names are rejected rather than silently turned into empty
  filenames.
