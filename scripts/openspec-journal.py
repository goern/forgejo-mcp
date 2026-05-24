#!/usr/bin/env python3
"""OpenSpec Journal — append-only interaction log for spec-driven changes.

Run with no arguments for the full usage contract.
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

VERSION = "0.2.0"
MAX_SUMMARY = 200

EVENT_PHASE = {
    "change.created": "proposal",
    "artifact.added": "authoring",
    "artifact.revised": "authoring",
    "mode.chosen": "apply",
    "task.start": "apply",
    "task.complete": "apply",
    "task.blocked": "apply",
    "verifier.result": "apply",
    "decision": "any",
    "handoff": "any",
    "archive": "archive",
    "skill.invoked": "any",
    "agent.spawned": "any",
    "context.compacted": "any",
    "turn.start": "any",
    "turn.end": "any",
}

REQUIRED_FIELDS = {
    "change.created": [],
    "artifact.added": ["ref"],
    "artifact.revised": ["ref", "input", "output"],
    "mode.chosen": ["mode", "input"],
    "task.start": ["ref"],
    "task.complete": ["ref", "mode", "input", "output"],
    "task.blocked": ["ref", "input", "output"],
    "verifier.result": ["ref", "note", "input", "output"],
    "decision": ["input", "output"],
    "handoff": ["input", "output"],
    "archive": [],
    "skill.invoked": ["name"],
    "agent.spawned": ["count", "kind"],
    "context.compacted": [],
    "turn.start": ["input"],
    "turn.end": ["output"],
}

NOTE_VALUES = {"verifier.result": {"pass", "concerns", "fail"}}
MODE_VALUES = {"mode.chosen": {"direct", "team"}}

EVENT_DESCRIPTION = {
    "change.created":    "OpenSpec change directory just came into being",
    "artifact.added":    "Wrote a new artifact file (proposal/design/specs/tasks)",
    "artifact.revised":  "Edited an existing artifact after it was first written",
    "mode.chosen":       "Apply phase started; declares direct vs team execution",
    "task.start":        "Began work on a specific task id from tasks.md",
    "task.complete":     "Finished a task; checkbox flipped to [x]",
    "task.blocked":      "Paused a task on an external dependency or unknown",
    "verifier.result":   "Independent verifier returned a verdict for a task",
    "decision":          "Load-bearing structural call inside a turn",
    "handoff":           "Last line before stopping; first line on resume",
    "archive":           "Change moved into openspec/changes/archive/",
    "skill.invoked":     "User-invoked skill ran against this change",
    "agent.spawned":     "Fan-out subagent group launched (e.g. team:debate)",
    "context.compacted": "PreCompact hook fired (autoemit, not by skills)",
    "turn.start":        "Bookend opened; logged BEFORE work in response to a prompt",
    "turn.end":          "Bookend closed; logged AFTER finishing the prompt",
}

FIELD_DESCRIPTION = {
    "ref":    "Task id, file path, or artifact id this event refers to",
    "input":  "What initiated this event (≤200 chars; the user/actor's ask)",
    "output": "What actually changed or was decided (≤200 chars)",
    "mode":   "Execution mode for the apply run: direct | team",
    "note":   "Verifier verdict: pass | concerns | fail",
    "name":   "Skill identifier (e.g. showboat, retro, priya)",
    "count":  "Integer count of subagents spawned in this fan-out",
    "kind":   "Label for the fan-out group (e.g. lens-scan, debate, spike)",
}

USAGE = """\
OpenSpec Journal — interaction log for spec-driven changes

WHAT
  Append-only JSONL at openspec/changes/<change>/journal.jsonl.
  Captures interaction points (decisions, task transitions, verifier
  results), not full session transcripts. Committed alongside code;
  travels with the change into
  openspec/changes/archive/<YYYY-MM-DD>-<change>/journal.jsonl
  at archive time.

ORDERING
  Events describe things that have happened, not things about to happen.
  Log AFTER the action completes. In particular, `change.created` is
  logged AFTER `openspec new change <name>` succeeds — the change
  directory must exist before the helper can append.

USAGE
  openspec-journal.py <change> <event> [k=v ...]
  openspec-journal.py <change> show [--limit N]
  openspec-journal.py <change> doctor
  openspec-journal.py --schema
  openspec-journal.py --version

EVENTS (fixed vocabulary)
  change.created     artifact.added     artifact.revised
  mode.chosen        task.start         task.complete
  task.blocked       verifier.result    decision
  handoff            archive            skill.invoked
  agent.spawned      context.compacted  turn.start
  turn.end

TURN BOOKENDING (when CWD is inside an active change)
  Log `turn.start input="<user ask, ≤200 chars>"` BEFORE doing work.
  Log `turn.end output="<what changed/was answered>"` AFTER finishing.
  Two writes per turn. Captures intent and result on every prompt,
  not only when files change. Skip only if no active change exists.

KEY=VALUE FIELDS
  Common to most events:
    ref=<id>         Task number, file path, or artifact id
    input=<text>     What the user/actor initiated, ≤200 chars (REJECTED if longer)
    output=<text>    What actually changed/resulted, ≤200 chars (REJECTED if longer)
    actor=<name>     Optional free-form: implementer | verifier | lead | user
    phase=<p>        Optional override; default derived from event

  Event-specific structured fields (machine-readable, no length limit):
    mode=<m>         direct | team        (mode.chosen, task.complete)
    note=<n>         pass | concerns | fail   (verifier.result)
    name=<skill>     showboat | retro | priya | …    (skill.invoked)
    count=<int>      Number of subagents spawned    (agent.spawned)
    kind=<label>     lens-scan | debate | spike | … (agent.spawned)
    agents=<int>     Optional on mode.chosen: subagent count for the run
    ctx_pct=<int>    Optional on context.compacted: context % at compaction

  Use --input-file PATH / --output-file PATH for long summaries (still
  rejected if >200 chars after newline normalization).

WRITING STYLE for input/output
  Verb + object + result. One sentence. No "I". No filler.

  GOOD  output="Verifier failed 3.2: missing snapshot for Line::Doctor."
  BAD   output="I ran the verifier and there was an issue with the test."

  GOOD  input="Added scenario 'export rejects empty CSV' to data-export."
  BAD   input="Made some changes to handle the edge case discussed."

  Too long? The journal references; it does not contain. Move long content
  to design.md or an ADR and point at it:
    output="See design.md §3 for full rationale."

LENGTH HANDLING
  By default, input/output >200 chars are REJECTED with exit 2 so the
  caller can rewrite shorter (no partial line is written). To force-write
  a truncated value anyway, pass --allow-truncate (revealed in the
  rejection error message).

EXAMPLE
  openspec-journal.py add-journal-skill task.complete \\
    ref=2.3 mode=team \\
    input="Implementer added journal helper + format spec." \\
    output="Verifier pass; 4 unit tests for truncation."

REQUIRED FIELDS PER EVENT
  Run with --schema for the full table.

EXIT CODES
  0  ok
  1  usage error
  2  validation error (doctor, length, or required-field check failed)

NEXT STEPS
  openspec-journal.py --schema           Show required-fields table
  openspec-journal.py <change> show      Last entries of a change
  openspec-journal.py <change> doctor    Validate journal.jsonl
"""


def find_openspec_root(start: Path) -> Path:
    """Walk upward from start to find a directory containing openspec/changes/."""
    cur = start.resolve()
    for parent in [cur, *cur.parents]:
        candidate = parent / "openspec" / "changes"
        if candidate.is_dir():
            return parent
    raise SystemExit(
        "Error: not inside an OpenSpec workspace.\n"
        "  No openspec/changes/ directory found in any parent of "
        f"{start}.\n  Run from inside a Spellkave checkout, or set CWD."
    )


def fail(code: int, msg: str) -> "NoReturn":  # type: ignore[name-defined]
    sys.stderr.write(msg + ("\n" if not msg.endswith("\n") else ""))
    sys.exit(code)


def parse_kv(args: list[str]) -> dict[str, str]:
    out: dict[str, str] = {}
    for raw in args:
        if "=" not in raw:
            raise SystemExit(
                f"Error: '{raw}' is not k=v form.\n"
                "  Each extra argument must be key=value."
            )
        k, v = raw.split("=", 1)
        if not k:
            raise SystemExit(f"Error: empty key in '{raw}'.")
        out[k] = v
    return out


def schema_table() -> str:
    rows = []
    for ev in EVENT_PHASE:
        req = ", ".join(REQUIRED_FIELDS[ev]) or "(none)"
        rows.append(
            f"  {ev:<18} phase={EVENT_PHASE[ev]:<10} required: {req}"
        )
        rows.append(f"      {EVENT_DESCRIPTION[ev]}")
    field_rows = [f"  {f:<8} {FIELD_DESCRIPTION[f]}" for f in FIELD_DESCRIPTION]
    extras = [
        "",
        "Field meanings:",
        *field_rows,
        "",
        "Constrained values:",
        "  mode={direct,team}    note={pass,concerns,fail}",
        "",
        "Length handling:",
        f"  input/output rejected at >{MAX_SUMMARY} chars (newlines stripped).",
        "  Use --allow-truncate to force-write a truncated value.",
    ]
    return "Event vocabulary:\n" + "\n".join(rows) + "\n" + "\n".join(extras)


def journal_path(root: Path, change: str) -> Path:
    change_dir = root / "openspec" / "changes" / change
    if change_dir.is_dir():
        return change_dir / "journal.jsonl"

    archive_root = root / "openspec" / "changes" / "archive"
    direct = archive_root / change
    if direct.is_dir():
        return direct / "journal.jsonl"
    if archive_root.is_dir():
        matches = sorted(
            p for p in archive_root.iterdir()
            if p.is_dir() and (p.name == change or p.name.endswith(f"-{change}"))
        )
        if len(matches) == 1:
            return matches[0] / "journal.jsonl"
        if len(matches) > 1:
            names = ", ".join(p.name for p in matches)
            raise SystemExit(
                f"Error: ambiguous archived change '{change}'.\n"
                f"  Multiple matches under archive/: {names}\n"
                "  Use the full directory name (with date prefix)."
            )
    raise SystemExit(
        f"Error: change '{change}' not found.\n"
        f"  Looked in {change_dir}\n"
        f"  and under {archive_root} (incl. date-prefixed entries)\n"
        "  Create the change directory first, or check the name."
    )


def cmd_add(
    change: str,
    event: str,
    kv: dict[str, str],
    json_out: bool,
    verbose: bool,
    allow_truncate: bool,
) -> int:
    if event not in EVENT_PHASE:
        suggest = ""
        # Cheap typo hint: prefix match.
        for known in EVENT_PHASE:
            if known.startswith(event[:4]):
                suggest = f"  Did you mean '{known}'?\n"
                break
        fail(
            2,
            f"Error: unknown event '{event}'.\n{suggest}"
            "  Run with no args for the vocabulary.",
        )

    missing = [f for f in REQUIRED_FIELDS[event] if f not in kv]
    if missing:
        fail(
            2,
            f"Error: event '{event}' missing required fields: "
            f"{', '.join(missing)}.\n"
            "  Run with --schema for the full table.",
        )

    if event in NOTE_VALUES:
        if kv.get("note") not in NOTE_VALUES[event]:
            fail(
                2,
                f"Error: note='{kv.get('note')}' invalid for {event}.\n"
                f"  Allowed: {sorted(NOTE_VALUES[event])}",
            )
    if event in MODE_VALUES:
        if kv.get("mode") not in MODE_VALUES[event]:
            fail(
                2,
                f"Error: mode='{kv.get('mode')}' invalid for {event}.\n"
                f"  Allowed: {sorted(MODE_VALUES[event])}",
            )

    record: dict[str, object] = {
        "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "phase": kv.pop("phase", EVENT_PHASE[event]),
        "event": event,
    }
    for key in ("ref", "mode", "note", "actor", "name", "kind", "count"):
        if key in kv:
            record[key] = kv.pop(key)
    over_limit: list[tuple[str, int]] = []
    truncated_fields: list[tuple[str, int]] = []
    for key in ("input", "output"):
        if key in kv:
            original = kv.pop(key)
            normalized = original.replace("\n", " ").replace("\r", " ").strip()
            if len(normalized) > MAX_SUMMARY:
                if not allow_truncate:
                    over_limit.append((key, len(normalized)))
                    continue
                truncated_fields.append((key, len(normalized)))
                record[key] = normalized[: MAX_SUMMARY - 1] + "…"
            else:
                record[key] = normalized
    if over_limit:
        details = ", ".join(f"{k}={n}" for k, n in over_limit)
        fail(
            2,
            f"Error: {details} chars exceed limit ({MAX_SUMMARY}).\n"
            "  Nothing written. Shorten the summary, or move long content\n"
            "  to design.md or an ADR and reference it:\n"
            '    output="See design.md §3 for full rationale."\n'
            "  Force-write the truncated value with --allow-truncate.",
        )
    for key, val in kv.items():
        record[key] = val

    root = find_openspec_root(Path.cwd())
    target = journal_path(root, change)
    line = json.dumps(record, ensure_ascii=False, separators=(",", ":")) + "\n"
    with target.open("a", encoding="utf-8") as fh:
        fh.write(line)

    if json_out:
        sys.stdout.write(line)
    else:
        print(f"appended {event} -> {target.relative_to(root)}")
        if truncated_fields:
            details = ", ".join(f"{k} ({n}->{MAX_SUMMARY})" for k, n in truncated_fields)
            print(f"  truncated under --allow-truncate: {details}")
        print()
        print("Next steps:")
        print("  Include journal.jsonl in your next git commit (piggyback).")
        print(f"  scripts/openspec-journal.py {change} show   Inspect last entries")
    return 0


def cmd_show(change: str, limit: int) -> int:
    root = find_openspec_root(Path.cwd())
    target = journal_path(root, change)
    if not target.exists():
        print(f"No journal yet at {target.relative_to(root)}.")
        return 0
    lines = target.read_text(encoding="utf-8").splitlines()
    tail = lines[-limit:]
    for raw in tail:
        try:
            rec = json.loads(raw)
        except json.JSONDecodeError:
            print(f"!! malformed: {raw[:120]}")
            continue
        head = f"{rec.get('ts','?')}  {rec.get('event','?'):<18}"
        ref = rec.get("ref")
        if ref:
            head += f"  ref={ref}"
        note = rec.get("note")
        if note:
            head += f"  note={note}"
        print(head)
        if rec.get("input"):
            print(f"  in : {rec['input']}")
        if rec.get("output"):
            print(f"  out: {rec['output']}")
    return 0


def cmd_doctor(change: str) -> int:
    root = find_openspec_root(Path.cwd())
    target = journal_path(root, change)
    if not target.exists():
        print(f"No journal at {target.relative_to(root)} (nothing to validate).")
        return 0

    errors, warnings = [], []
    last_ts = ""
    turn_starts = 0
    turn_ends = 0
    for n, raw in enumerate(target.read_text(encoding="utf-8").splitlines(), 1):
        if not raw.strip():
            warnings.append(f"line {n}: blank line")
            continue
        try:
            rec = json.loads(raw)
        except json.JSONDecodeError as e:
            errors.append(f"line {n}: not valid JSON ({e.msg})")
            continue
        ev = rec.get("event")
        if ev not in EVENT_PHASE:
            errors.append(f"line {n}: unknown event '{ev}'")
            continue
        if ev == "turn.start":
            turn_starts += 1
        elif ev == "turn.end":
            turn_ends += 1
        for f in REQUIRED_FIELDS[ev]:
            if f not in rec:
                errors.append(f"line {n} ({ev}): missing required field '{f}'")
        if ev in NOTE_VALUES and rec.get("note") not in NOTE_VALUES[ev]:
            errors.append(f"line {n}: note='{rec.get('note')}' invalid for {ev}")
        if ev in MODE_VALUES and rec.get("mode") not in MODE_VALUES[ev]:
            errors.append(f"line {n}: mode='{rec.get('mode')}' invalid for {ev}")
        for f in ("input", "output"):
            v = rec.get(f)
            if isinstance(v, str) and len(v) > MAX_SUMMARY:
                warnings.append(f"line {n}: {f} exceeds {MAX_SUMMARY} chars")
        ts = rec.get("ts")
        if not isinstance(ts, str):
            errors.append(f"line {n}: 'ts' must be a string, got {type(ts).__name__}")
        else:
            if last_ts and ts < last_ts:
                warnings.append(f"line {n}: ts '{ts}' precedes previous '{last_ts}'")
            last_ts = ts

    # Turn bookending symmetry: each turn.start should have a matching
    # turn.end. Allow turn_starts == turn_ends + 1 for an in-progress turn.
    if turn_starts > turn_ends + 1:
        warnings.append(
            f"turn bookending: {turn_starts} turn.start vs "
            f"{turn_ends} turn.end ({turn_starts - turn_ends} unclosed; "
            "expected at most 1 in-progress turn)"
        )
    elif turn_ends > turn_starts:
        warnings.append(
            f"turn bookending: {turn_ends} turn.end exceeds "
            f"{turn_starts} turn.start (orphan turn.end events)"
        )

    rel = target.relative_to(root)
    if errors:
        print(f"FAIL  {rel}")
        for e in errors:
            print(f"  ✗ {e}")
    else:
        print(f"OK    {rel}")
    for w in warnings:
        print(f"  ⚠ {w}")
    print(f"\nSummary: {len(errors)} error(s), {len(warnings)} warning(s)")
    print(f"Turns: {turn_starts} start / {turn_ends} end")
    return 2 if errors else 0


def main(argv: list[str]) -> int:
    if len(argv) == 0:
        sys.stdout.write(USAGE)
        return 0

    # Split argv into flags and positionals manually so flags can appear
    # anywhere on the line (before, between, or after positionals).
    NO_VALUE = {
        "--schema", "--version", "--json", "--verbose", "-v",
        "-h", "--help", "--allow-truncate",
    }
    TAKES_VALUE = {"--limit", "--input-file", "--output-file"}
    flag_argv: list[str] = []
    positionals: list[str] = []
    i = 0
    while i < len(argv):
        tok = argv[i]
        if tok in NO_VALUE:
            flag_argv.append(tok)
            i += 1
        elif tok in TAKES_VALUE:
            if i + 1 >= len(argv):
                sys.stderr.write(f"Error: {tok} needs a value.\n")
                return 1
            flag_argv.extend([tok, argv[i + 1]])
            i += 2
        elif tok.startswith("--") and "=" in tok and tok.split("=", 1)[0] in TAKES_VALUE:
            flag_argv.append(tok)
            i += 1
        elif tok.startswith("-") and tok != "-" and "=" not in tok:
            sys.stderr.write(
                f"Error: unknown flag '{tok}'. Run with no args for usage.\n"
            )
            return 1
        else:
            positionals.append(tok)
            i += 1

    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("--schema", action="store_true")
    parser.add_argument("--version", action="store_true")
    parser.add_argument("--json", action="store_true")
    parser.add_argument("--verbose", "-v", action="store_true")
    parser.add_argument("--limit", type=int, default=5)
    parser.add_argument("--input-file", type=str)
    parser.add_argument("--output-file", type=str)
    parser.add_argument("--allow-truncate", action="store_true")
    parser.add_argument("-h", "--help", action="store_true")
    args = parser.parse_args(flag_argv)

    if args.help:
        sys.stdout.write(USAGE)
        return 0
    if args.version:
        print(VERSION)
        return 0
    if args.schema:
        print(schema_table())
        return 0

    if len(positionals) < 2:
        sys.stderr.write(
            "Error: need at least <change> <event|verb>.\n"
            "  Run with no args for usage.\n"
        )
        return 1

    change, verb, *kv_args = positionals

    if verb == "show":
        return cmd_show(change, args.limit)
    if verb == "doctor":
        return cmd_doctor(change)

    kv = parse_kv(kv_args)
    if args.input_file and "input" not in kv:
        kv["input"] = Path(args.input_file).read_text(encoding="utf-8")
    if args.output_file and "output" not in kv:
        kv["output"] = Path(args.output_file).read_text(encoding="utf-8")
    return cmd_add(
        change, verb, kv, args.json, args.verbose, args.allow_truncate
    )


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv[1:]))
    except SystemExit:
        raise
    except Exception as e:  # noqa: BLE001
        sys.stderr.write(f"Error: {e}\n")
        sys.exit(1)
