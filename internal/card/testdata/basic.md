---
schema: 1
id: 01923f44-5a06-7d2e-8c9f-1b2d3e4f5a6b
type: basic
created: 2026-01-15T10:30:00Z
tags: [golang, testing]
---

## Front

What is the Go testing convention for table-driven tests?

## Back

Define a slice of struct cases, range over it, and run `t.Run` per case.
