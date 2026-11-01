# INIT-026 architecture

Provenance: INIT-026/DESIGN-DIAGRAMS. Written from the specs and the code on branch `tutor-data-loop`. This app has no in-process agent fleet, so the agent diagram is the outside tutor calling MCP.

## System context

Grok, or any OAuth MCP client signed in as the learner, calls the Hebrew server. The human tutor never signs in. Recordings and transcripts stay with the caller.

```mermaid
flowchart LR
  learner[Learner]
  grok[Tutor LLM]
  app[Hebrew server]
  db[(SQLite)]
  learner --> app
  grok -->|OAuth as the learner| app
  app --> db
```

Sources: INIT-026/SPEC-001, INIT-026/SPEC-007, INIT-026/SPEC-008, INIT-026/SPEC-009.

## Container

One Go process serves the pages and the MCP tools. Both use `internal/studyday` for the learner's day. The daily and weekly jobs run in the same binary.

```mermaid
flowchart TB
  web[Pages]
  mcp[MCP tools]
  day[studyday]
  jobs[Daily and weekly jobs]
  ev[evidence.Recorder]
  db[(SQLite)]
  web --> day
  mcp --> day
  jobs --> day
  mcp --> ev
  web --> db
  mcp --> db
  jobs --> db
  ev --> db
```

Sources: INIT-026/SPEC-003, INIT-026/SPEC-004, INIT-026/SPEC-005, INIT-026/SPEC-006.

## Data

Migration 020 adds the zone, the taught-word rows, the correction days, and the note role.

```mermaid
erDiagram
  users ||--o{ taught_words : learns
  users ||--o{ learner_mistakes : corrects
  users ||--o{ coach_notes : notes
  lexemes ||--o{ taught_words : taught
  users {
    text time_zone
  }
  taught_words {
    text taught_on
    text cards_created
  }
  learner_mistakes {
    text first_on
    text last_on
  }
  coach_notes {
    text author_role
  }
```

Source: INIT-026/SPEC-002.

## Sequence: after a session

```mermaid
sequenceDiagram
  participant G as Tutor LLM
  participant M as MCP
  participant E as Recorder
  participant D as SQLite
  G->>M: get_study_digest
  M->>D: read own rows
  M-->>G: struggle, corrections, taught
  G->>M: record_taught_words on=today
  M->>E: Taught
  E->>D: new cards due at next 4:00
  G->>M: record_mistakes on=today
  M->>D: first_on / last_on
```

The recorder inserts new cards only. It does not grade an existing card. Sources: INIT-026/SPEC-005, INIT-026/SPEC-008, INIT-026/SPEC-009.

## Deployment

The live server is the `hebrew` Deployment in namespace `hebrew`. `TZ` is the app zone when a learner has none. This initiative does not change the Deployment. A learner's zone is a row, set with `set_time_zone`.

Source: INIT-026/SPEC-003, INIT-026/SPEC-007.
