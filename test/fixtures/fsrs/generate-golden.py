"""
Golden values for shared/player/fsrs.js (ADR-0009 §2).

Run OUTSIDE the repository, in a throwaway virtual environment, so the
reference implementation never becomes a dependency of this project:

    python -m venv venv
    venv/Scripts/python -m pip install fsrs==6.3.2      (venv/bin/python on Linux/macOS)
    venv/Scripts/python test/fixtures/fsrs/generate-golden.py test/fixtures/fsrs/fsrs6-golden.json

Then run crosscheck-ts-fsrs.cjs (same folder) to add the ts-fsrs cross-check.
npm test never runs this file. test/fsrs.test.js only reads the JSON.

Stability, difficulty, retrievability, and the per-grade reference intervals
come from py-fsrs. The `interval` and `previewIntervals` columns then apply
ADR-0009 §2's interval rules: Again is due the next day; Good is at least
Hard + 1 and Easy at least Good + 1; every interval is 1 to 365 days.

Authored-by-spec: INIT-006/SPEC-004
"""

import json
import math
import sys
from datetime import datetime, timedelta, timezone
from importlib.metadata import version

from fsrs import Card, Rating, Scheduler
from fsrs.scheduler import DEFAULT_PARAMETERS

ADR_0009_PARAMETERS = (
    0.212, 1.2931, 2.3065, 8.2956, 6.4133, 0.8334, 3.0194, 0.001, 1.8722, 0.1666,
    0.796, 1.4835, 0.0614, 0.2629, 1.6483, 0.6014, 1.8729, 0.5425, 0.0912, 0.0658,
    0.1542,
)
DESIRED_RETENTION = 0.9
MIN_INTERVAL = 1
MAX_INTERVAL = 365
PACKAGE_VERSION = "6.3.2"

if tuple(DEFAULT_PARAMETERS) != ADR_0009_PARAMETERS:
    sys.exit("py-fsrs default parameters differ from the ADR-0009 §2 list")
if version("fsrs") != PACKAGE_VERSION:
    sys.exit("expected fsrs==" + PACKAGE_VERSION + ", found " + version("fsrs"))

SCHEDULER = Scheduler(
    parameters=ADR_0009_PARAMETERS,
    desired_retention=DESIRED_RETENTION,
    learning_steps=(),
    relearning_steps=(),
    maximum_interval=MAX_INTERVAL,
    enable_fuzzing=False,
)

START = datetime(2026, 1, 5, 12, 0, tzinfo=timezone.utc)

# Each step is (grade, when). when: "start"; "due" (the previous step's due,
# using the ADR interval); "due+Nd" (N days after that due); "+Nh" / "+Nd"
# (N hours / days after the previous review).
CASES = [
    ("new-good-run", "Good on a new card, then Good on every due date.",
     [(3, "start"), (3, "due"), (3, "due"), (3, "due"), (3, "due"), (3, "due")]),
    ("new-again-recover", "Again on a new card, then recovery on each due date.",
     [(1, "start"), (3, "due"), (3, "due"), (2, "due"), (4, "due")]),
    ("new-hard-then-easy", "Hard on a new card, Hard again, then Easy and Good.",
     [(2, "start"), (2, "due"), (4, "due"), (3, "due")]),
    ("lapse-after-learning", "Three Goods, a lapse on the due date, then recovery.",
     [(3, "start"), (3, "due"), (3, "due"), (1, "due"), (3, "due"), (3, "due")]),
    ("mature-lapse",
     "A card with long stability is forgotten. Stability follows FSRS; "
     "ADR-0009 puts an Again card on the next day.",
     [(4, "start"), (4, "due"), (3, "due"), (1, "due"), (3, "due"), (3, "due")]),
    ("same-day-flashcard",
     "Explicit grades later on the same day use the FSRS same-day formula.",
     [(3, "start"), (1, "+2h"), (3, "+1h"), (3, "+1d"), (4, "+3h"), (2, "+1h"), (3, "due")]),
    ("lapse-cap",
     "Again on a new card, then Again a year later: stability after the lapse "
     "is capped at S / e^(w17 * w18).",
     [(1, "start"), (1, "+365d"), (3, "due")]),
    ("repeated-same-day-again",
     "Again twice on a new card in one session, then Hard and Good: intervals under "
     "one day become one day before the ordering rule.",
     [(1, "start"), (1, "+1h"), (2, "+1h"), (3, "+1h"), (3, "+1d")]),
    ("late-and-early", "A review twelve days late, then one long before the due date.",
     [(3, "start"), (3, "due"), (3, "due+12d"), (2, "+1d"), (3, "due")]),
    ("interval-cap", "Easy on every due date until the interval reaches 365 days.",
     [(4, "start"), (4, "due"), (4, "due"), (4, "due"), (3, "due")]),
]


def iso(moment):
    return moment.astimezone(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def unrounded_interval(stability):
    return (stability / SCHEDULER._FACTOR) * (DESIRED_RETENTION ** (1 / SCHEDULER._DECAY) - 1)


def reference_outcomes(card, at):
    """Review a copy of the card with each grade. The card itself is not changed."""
    out = {}
    for grade in (1, 2, 3, 4):
        reviewed, _ = SCHEDULER.review_card(card, Rating(grade), review_datetime=at)
        days = (reviewed.due - at) / timedelta(days=1)
        if days != int(days):
            sys.exit("reference interval is not whole days")
        fraction = unrounded_interval(reviewed.stability) % 1
        if abs(fraction - 0.5) < 1e-6:
            sys.exit("an interval sits on a .5 rounding boundary; JS and Python round it differently")
        out[grade] = {"card": reviewed, "interval": int(days)}
    return out


def adr_intervals(reference):
    hard = min(reference[2], reference[3])
    good = max(reference[3], hard + 1)
    easy = max(reference[4], good + 1)
    raw = {1: MIN_INTERVAL, 2: hard, 3: good, 4: easy}
    return {grade: min(max(days, MIN_INTERVAL), MAX_INTERVAL) for grade, days in raw.items()}


def next_time(when, previous_at, previous_due):
    if when == "start":
        return START
    if when == "due":
        return previous_due
    if when.startswith("due+") and when.endswith("d"):
        return previous_due + timedelta(days=int(when[4:-1]))
    if when.startswith("+") and when.endswith("h"):
        return previous_at + timedelta(hours=int(when[1:-1]))
    if when.startswith("+") and when.endswith("d"):
        return previous_at + timedelta(days=int(when[1:-1]))
    sys.exit("unknown step time: " + when)


def build_case(name, about, steps):
    card = Card()
    previous_at = None
    previous_due = None
    rows = []
    for grade, when in steps:
        at = next_time(when, previous_at, previous_due)
        has_memory = card.stability is not None
        elapsed = None if card.last_review is None else (at - card.last_review).days
        retrievability = SCHEDULER.get_card_retrievability(card, at) if has_memory else None
        outcomes = reference_outcomes(card, at)
        previews = adr_intervals({g: o["interval"] for g, o in outcomes.items()})
        chosen = outcomes[grade]
        notes = []
        if previews[grade] != chosen["interval"]:
            if grade == 1:
                notes.append("ADR-0009 §2: an Again card is due the next day")
            else:
                notes.append("ADR-0009 §2: Good is at least Hard + 1 and Easy at least Good + 1")
        row = {
            "at": iso(at),
            "grade": grade,
            "elapsedDays": elapsed,
            "sameDay": bool(has_memory and elapsed is not None and elapsed < 1),
            "retrievability": retrievability,
            "previewIntervals": {str(g): previews[g] for g in (1, 2, 3, 4)},
            "referenceIntervals": {str(g): outcomes[g]["interval"] for g in (1, 2, 3, 4)},
            "stability": chosen["card"].stability,
            "difficulty": chosen["card"].difficulty,
            "interval": previews[grade],
            "referenceInterval": chosen["interval"],
        }
        if notes:
            row["note"] = "; ".join(notes)
        rows.append(row)
        card = chosen["card"]
        previous_at = at
        previous_due = at + timedelta(days=previews[grade])
    return {"name": name, "about": about, "start": iso(START), "steps": rows}


def main():
    fixture = {
        "about": (
            "Golden values for shared/player/fsrs.js. stability, difficulty, retrievability, "
            "and referenceIntervals come from the reference implementation. interval and "
            "previewIntervals apply ADR-0009 §2's interval rules on top of it: Again is due "
            "the next day; Good is at least Hard + 1 and Easy at least Good + 1; 1 to 365 days. "
            "Times are UTC, and the test pins the local time zone to UTC so a calendar day "
            "is a UTC day."
        ),
        "formulaSource": {
            "page": "open-spaced-repetition, \"The Algorithm\" (FSRS-6)",
            "url": "https://github.com/open-spaced-repetition/awesome-fsrs/wiki/The-Algorithm",
            "retrieved": "2026-10-09",
            "adr": "docs/decisions/ADR-0009-learner-model-and-language-program.md §2",
        },
        "implementation": {
            "name": "py-fsrs (PyPI package fsrs)",
            "version": version("fsrs"),
            "url": "https://pypi.org/project/fsrs/" + version("fsrs") + "/",
            "fsrsVersion": "FSRS-6",
            "settings": {
                "learning_steps": [],
                "relearning_steps": [],
                "desired_retention": DESIRED_RETENTION,
                "maximum_interval": MAX_INTERVAL,
                "enable_fuzzing": False,
            },
            "ranOutsideRepository": True,
            "generator": "test/fixtures/fsrs/generate-golden.py",
            "pythonVersion": sys.version.split()[0],
        },
        "parameters": list(ADR_0009_PARAMETERS),
        "desiredRetention": DESIRED_RETENTION,
        "intervalDays": {"min": MIN_INTERVAL, "max": MAX_INTERVAL},
        "cases": [build_case(name, about, steps) for name, about, steps in CASES],
    }
    if len(sys.argv) != 2:
        sys.exit("usage: generate-golden.py <output.json>")
    with open(sys.argv[1], "w", encoding="utf-8", newline="\n") as out:
        json.dump(fixture, out, indent=2, ensure_ascii=False)
        out.write("\n")


if __name__ == "__main__":
    main()
