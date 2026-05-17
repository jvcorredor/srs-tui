---
title: Quick start
description: Write your first card and run a review session with srs.
---

This guide walks you from a fresh install to your first completed review
session. If you do not have `srs` yet, follow the
[installation guide](/srs-tui/install/) first.

## 1. Create a card file

`srs` cards are plain Markdown files. Create a directory for your deck and add
a card:

```sh
mkdir -p ~/cards
$EDITOR ~/cards/geography.md
```

A card is a question and an answer separated by a divider:

```markdown
# What is the capital of France?

---

Paris
```

You can keep many cards in a single file, or split them across as many files
as you like — `srs` reads every Markdown file it is pointed at.

## 2. Start a review session

Launch the TUI against your deck:

```sh
srs ~/cards
```

`srs` selects the cards that are due according to the FSRS schedule and shows
them one at a time.

## 3. Review a card

For each card:

1. Read the question and recall the answer.
2. Reveal the answer.
3. Grade how well you remembered it. FSRS uses your grade to decide when the
   card should next appear.

The further out a card is scheduled, the less often you will see it — that is
spaced repetition working for you.

## 4. Finish the session

When no cards remain due, `srs` shows a session summary and exits. Your
progress is saved automatically, so you can pick up where you left off the
next time a card comes due.

## Where to go next

- Keep adding cards as Markdown files — they are yours to edit and
  version-control.
- Run `srs` daily so the FSRS scheduler can keep your reviews on track.
- Browse the [project on GitHub](https://github.com/jvcorredor/srs-tui) for
  the full roadmap and to report issues.
