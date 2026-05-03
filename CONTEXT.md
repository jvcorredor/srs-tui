# Context

Single source of truth for domain language in srs-tui.

## Canonical Terms

### Deck
A directory on disk that contains `.md` card files. Each deck is a self-contained folder inside the decks root directory. Discovered at runtime by walking the filesystem.

### Card
A markdown file with a YAML frontmatter block and `## Front` / `## Back` content sections. Cards are the atomic unit of study. Each card has a unique ID, a type (`basic` or `cloze`), creation timestamp, optional tags, and scheduling metadata persisted in its frontmatter.

### Review
An interactive TUI session where cards from a single deck are presented one at a time. The user sees the front first, reveals the back, then assigns a rating. The session continues until the queue is exhausted.

### Rating
An integer score assigned during review on a 1–4 scale:
- `1` = Again (failed recall)
- `2` = Hard
- `3` = Good
- `4` = Easy
Ratings feed the FSRS scheduler to compute the next due date and card state.

### State
The learning state of a card in the FSRS lifecycle:
- `new` – never reviewed
- `learning` – recently introduced, still in the initial learning phase
- `review` – graduated, being maintained through spaced repetition
- `relearning` – lapsed after a failed review and is being re-learned

## Supporting Concepts

### Queue
A shuffled slice of cards produced from a deck at the start of a review session. Built by walking the deck directory, parsing every `.md` file into a card, and randomizing the order.

### LogEntry
A single line in a JSON Lines file that records one review event. Captures timestamp, card ID, rating, duration, and the card's scheduling state before and after the rating.

### Store
Per-deck persistence layer. Manages the review log (append-only JSON Lines) and atomic card file rewrites after each rating. Each deck gets its own store instance keyed by a slug derived from the deck directory name.

### FSRS
Free Spaced Repetition Scheduler. The algorithm that converts a card's current state and a user rating into the next due date, stability, difficulty, and state. Exposed in the project as a thin wrapper around the `go-fsrs` library.

## File Layout

```
$XDG_DATA_HOME/srs/decks/          # Decks root (user cards)
$XDG_CONFIG_HOME/srs/config.toml   # Application config
$XDG_STATE_HOME/srs/<deck>.jsonl    # Per-deck review logs
```

## Relationships

- A **Deck** contains many **Cards**.
- A **Review** operates on one **Deck**, producing a **Queue** of its cards.
- Each card interaction yields a **Rating**, which is fed to **FSRS** to compute the next **State**.
- The **Store** persists both the **LogEntry** and the updated card file.
