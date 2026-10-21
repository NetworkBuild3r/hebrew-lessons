# INIT-015 Architecture: v3 Adaptive Learning (`INIT-015/DESIGN-DIAGRAMS`)

Diagrams are grounded in the specs and the code at `v3` `844f16d`. Each cites its source specs. Included: system context, container, data model, two sequences, data flow with trust boundaries, and deployment. Left out: component (C4 L3) and agent topology, because the app is one Go binary and the only agent is an external MCP client.

## 1. System context (C4 L1) · SPEC-001, SPEC-021, SPEC-022

```mermaid
flowchart LR
    learner([Learner<br/>phone or desktop browser])
    admin([Operator / admin])
    ai([AI coach<br/>Cursor Automation or any MCP client])
    eleven[(ElevenLabs TTS<br/>eleven_v3)]
    google[(Google sign-in<br/>OAuth)]
    app[[Hebrew app v3<br/>Go + templ + htmx + SQLite]]

    learner -- "HTTPS: placement, Today, review, progress" --> app
    admin -- "HTTPS + MCP admin session or static token" --> app
    ai -- "MCP over HTTPS<br/>learner data in, lessons out" --> app
    app -- "audio-ensure and generate_audio only" --> eleven
    app -- "sign-in" --> google
```

The AI never holds a learner session. It reads the learner model and a `LessonBrief`, writes a unit with a `briefId`, and the server validates and publishes it (D5, no human step).

## 2. Container (C4 L2) · SPEC-005, SPEC-007, SPEC-013 to SPEC-015, SPEC-019 to SPEC-021, SPEC-023

One process, one SQLite file on one volume. Packages are the containers.

```mermaid
flowchart TB
    subgraph proc[hebrew process]
        web[web<br/>handlers, templ views, static CSS/JS<br/>shell, placement, activity, progress]
        placement[placement<br/>engine: 200-word deck, bands]
        activity[activity + session<br/>skill-aware session mix]
        evidence[evidence<br/>one attempt recorder, grading rule]
        learner[learner<br/>states, diagnosis, Summary]
        jobs[jobs<br/>scheduler: daily snapshot, weekly report]
        brief[brief<br/>LessonBrief, coach notes]
        units[units<br/>validator, brief rules, no_markup]
        mcpsrv[mcp + mcpstore<br/>role policy, rate limit, tools]
        auth[auth<br/>sessions, CSRF, OAuth]
        audio[audio<br/>clip store, audio-ensure]
    end
    db[(SQLite<br/>/data, one file)]
    clips[(audio_assets<br/>mp3 clips)]

    web --> placement & activity & auth
    placement --> evidence
    activity --> evidence
    evidence --> db
    learner --> db
    jobs --> learner
    jobs --> db
    mcpsrv --> learner & brief & units
    brief --> db
    units --> db
    web --> audio
    audio --> clips
    clips --- db
```

## 3. Data model (ERD) · SPEC-003, SPEC-004, SPEC-006, SPEC-014, SPEC-019, SPEC-020, SPEC-023 (migrations 006 and 008 to 011)

Migration 007 is the OAuth client change that shipped on `origin/v3` first (D9). Every learner table carries `user_id`; there is no cross-learner join in any query.

```mermaid
erDiagram
    users ||--o{ attempts : "records"
    users ||--o{ exposures : "sees"
    users ||--o{ daily_snapshots : "has one per day"
    users ||--o{ regression_events : "slips"
    users ||--o{ weekly_reports : "has one per week"
    users ||--o{ lesson_briefs : "is given"
    users ||--o{ coach_notes : "gets"
    users ||--o{ unit_reports : "files"
    users ||--o{ placement_v3_runs : "sits"
    placement_v3_runs ||--o{ placement_items : "deals"
    lesson_briefs ||--o| units : "used_by_unit_id, one use"
    units ||--o{ publish_audit : "audited"
    attempts }o--|| cards : "grades"
    job_state { text job PK }
    attempts { int user_id FK int card_id text skill text outcome }
    daily_snapshots { int user_id FK text day text skill text state_counts }
    lesson_briefs { text id PK int user_id FK int used_by_unit_id }
    placement_items { int run_id FK int seq text part text band }
```

`job_state` holds the daily and weekly job locks. `cards_new` (006) rebuilt `cards` with a skill direction; `confusion_misses_new` (009) and `regression_events_new` (010) are table rebuilds.

## 4. Sequence: attempt to published lesson · SPEC-005, SPEC-013, SPEC-014, SPEC-019, SPEC-020, SPEC-021

```mermaid
sequenceDiagram
    participant L as Learner
    participant W as web
    participant E as evidence
    participant J as daily job
    participant M as learner model
    participant A as AI coach (MCP)
    participant S as server (units + brief)
    L->>W: answer an activity
    W->>E: Record(attempt, user from session)
    E->>E: grading rule, update card
    J->>M: nightly snapshot per learner and skill
    M-->>J: states, regressions
    A->>S: get_learner_model, get_progress, get_lesson_brief
    S-->>A: own-data only, brief with new-word targets
    A->>S: put_unit(scope learner, briefId, publish true)
    S->>S: ValidateWithBrief (markup, coverage, known share)
    alt valid
        S->>S: AutoPublishLearner, claim brief in one tx
        S-->>L: lesson appears on Today
    else invalid
        S-->>A: issues list, unit stays a draft
    end
```

## 5. Sequence: placement with no feedback (D1) · SPEC-007, SPEC-012

```mermaid
sequenceDiagram
    participant L as Learner
    participant W as web
    participant P as placement engine
    participant E as evidence
    L->>W: start
    W->>P: Start(userID)
    loop letters, listening, reading, optional typing
        P-->>W: item (no correctness field, CorrectIdx stripped)
        L->>W: answer or replace (max 3 per item)
        W->>P: Answer(userID, runID, item, choice)
        P->>P: judge from the stored item
        P->>E: attempt (right answers become cards, D2)
    end
    P-->>L: Results and review: misses shown only now
```

## 6. Data flow and trust boundaries · SPEC-002, SPEC-021, SPEC-023; threat model in the initiative doc section 8

```mermaid
flowchart LR
    subgraph untrusted[Untrusted]
        browser[Browser]
        mcpc[MCP client / AI]
    end
    subgraph edge[Edge checks]
        csrf[CSRF + Origin<br/>every non-safe method]
        csp[CSP, no unsafe-inline scripts]
        role[Role policy<br/>admin-only by default]
        rate[60 calls / minute<br/>per caller]
    end
    subgraph trusted[Trusted: one process]
        h[handlers and tools]
        v[validator<br/>no_markup, coverage, scope]
        d[(SQLite)]
    end
    browser --> csrf --> csp --> h
    mcpc --> role --> rate --> h
    h --> v --> d
    h -. "never" .-> secrets[(TTS keys, tokens:<br/>env only, never logged)]
```

Learner scope is forced server-side from the session or token. A learner naming another `learnerId` is refused before the handler runs. The read-only smoke token is a learner principal with writes removed.

## 7. Deployment · SPEC-002, SPEC-025

```mermaid
flowchart LR
    dev[push to v3] --> ci[CI on k8-build<br/>test, build, smoke]
    ci --> ghcr[(ghcr.io/networkbuild3r/hebrew<br/>sha-commit)]
    ghcr --> iu1[Image Updater<br/>hebrew]
    ghcr --> iu2[Image Updater<br/>hebrew-next]
    iu1 --> prod[ns hebrew<br/>Recreate, 1 replica<br/>PVC hebrew-v2-data]
    iu2 --> stg[ns hebrew-next<br/>PVC hebrew-next-data]
    prod --> m[init container migrates<br/>006 and 008 to 011]
    stg --> m2[init container migrates]
```

Both updaters track the same image, so staging is not a separate stage today (SPEC-025 documents the pause and a real staging dry run). Migrations run in the init container; a failing one shows as a stuck init container. The old image cannot read a migrated database, so rollback is the previous tag plus the backup file. Hosts: `hebrew.ibhacked.us`, `hebrew-next.ibhacked.us`, MCP at `hebrew-mcp.ibhacked.us`. CI never publishes `ghcr.io/networkbuild3r/hebrew-lessons`.
