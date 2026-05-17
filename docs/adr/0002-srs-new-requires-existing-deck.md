# 2. `srs new` requires an existing deck

Date: 2026-05-17

## Status

Accepted

## Context

`srs new <deck> <name>` creates a card inside a deck. Originally the command
called `os.MkdirAll` on the deck path, so an unrecognised `<deck>` argument was
treated as a brand-new deck and created on the spot.

This is a silent-failure hazard. A typo — `srs new spnaish hola` instead of
`spanish` — does not fail. It creates a stray `spnaish` deck, writes the card
into it, and opens the editor. The user only discovers the mistake later, when
the expected deck is missing a card and an unwanted deck has appeared. There is
no signal at the moment the mistake is made.

Deck creation is already a deliberate, explicit action available through
`srs deck create` and the TUI `N` keybinding. `srs new` having a second,
implicit path to deck creation gives the command two jobs and makes the
dangerous outcome indistinguishable from the intended one.

## Decision

`srs new` only ever adds cards to decks that **already exist**.

- The `<deck>` argument is slugified (see ADR 0001) and matched against the
  existing deck directories under the decks root.
- If the deck exists, the card is created as before.
- If the deck does not exist, `srs new` does **not** create it. It returns a
  plain runtime error (exit code 1 — not a usage error, because the arguments
  are well-formed). The message is actionable:
  - if an existing deck is within a small edit distance of the argument, it
    appends `Did you mean "<deck>"?`;
  - otherwise it lists the decks that do exist;
  - if there are no decks at all, it tells the user to run `srs deck create`.
- The edit-distance match uses a hand-rolled Levenshtein helper — no new
  third-party dependency.
- There is no `--create-deck` flag. `srs new` has exactly one job and never
  turns a free-text argument into a directory.

## Consequences

- A mistyped deck name fails loudly and immediately, with a suggestion, instead
  of silently creating a stray deck.
- This is a **behaviour change**. Anyone scripting `srs new` against a
  not-yet-created deck must now run `srs deck create` first; the previous
  implicit-creation behaviour is gone.
- Deck creation lives in exactly one place per surface (`srs deck create`, the
  TUI `N` keybinding), making the codebase easier to reason about.
- `srs new` still refuses to overwrite an existing card file.
