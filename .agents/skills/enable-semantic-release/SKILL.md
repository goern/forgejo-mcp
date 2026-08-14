---
name: enable-semantic-release
description: Grant b4mad-release-agent what it needs to run semantic-release on a Forgejo repo, install `just release`/`just preflight` into that repo's justfile and a short how-to into its AGENTS.md, then print the oc command that runs the release. Use when asked to "enable semantic-release for <repo>", to onboard a repo to the release Job, to add release targets to a justfile, or when a release push is rejected with "not allowed to push to branch".
---

# enable-semantic-release &lt;repo_url&gt;

Wraps `enable-semantic-release.py` — read its module docstring before
improvising; every check in it exists because it actually bit.

This skill is **self-contained**: the script and the Job manifest it edits both
ship in this directory, and the script resolves the manifest against its own
path, never the cwd. Nothing is downloaded at run time.

```
enable-semantic-release/
  SKILL.md
  enable-semantic-release.py     # forge-agents symlinks this at its root
  job-semantic-release.yaml      # and at openshift/job-semantic-release.yaml
  justfile-block.just            # the just targets it installs
  agents-md-block.md             # the how-to it installs into AGENTS.md
```

Those two symlinks are why there is one copy of each file rather than two: edit
the version here and the checkout follows. ⚠️ Do not "fix" a drifting copy by
replacing a symlink with a file.

Run it by its own path; the commands below use the `./` form that works from a
forge-agents checkout:

```bash
"$(dirname "$0")"/enable-semantic-release.py <url>    # from anywhere
```

It does **three** things, named `grant`, `justfile` and `agents-md`, and any
subset can be run alone with `--only` / `--skip`:

1. **`grant`** — the four-part chain the docstring walks through. Needs `oc`,
   logged in to the cluster holding namespace `b4mad-forgejo`: the admin and
   agent Secrets are read live, and the Forgejo it grants on is the one in that
   cluster. Without `oc` this part stops and changes nothing.
2. **`justfile`** — `preflight`, `release`, `release-dry`, `clean-jobs`
   appended to the target repo's justfile.
3. **`agents-md`** — a ten-line "how you release here" appended to its
   AGENTS.md. It points at the targets rather than restating them.

(2) and (3) are **managed blocks**: same fence, same version, same hash, same
rules, one implementation (`class Managed`). They need nothing but git, which
is why `--skip grant` is the half you can exercise anywhere.

## When to use

- "enable semantic-release for `<repo>`" / "let the release agent release X"
- A release run failed with
  `Forgejo: User 'b4mad-release-agent' is not allowed to push to branch 'main'`
- You want to know *why* a repo is not releasable without changing anything
  (`--dry-run`)
- A repo is already granted and just wants the targets or the docs
  (`--only justfile,agents-md`)

Do **not** use it to run a release on a repo that is already enabled — that is
just the `oc create` at the end.

## Input

One argument, an SSH clone URL in exactly this form:

```
ssh://git@git.b4mad.industries:2222/<owner>/<repo>.git
```

The form is required, not merely accepted: it is the literal string the Job's
`REPO_URL` takes, so a URL the script blesses can be pasted into the manifest
unchanged. If the user gives an HTTPS URL or `owner/repo`, rewrite it to this
form and say that you did.

## Instructions

1. **Survey first, always.** Run the dry run and show the plan:

   ```bash
   ./enable-semantic-release.py <url> --dry-run
   ```

   It changes nothing. Exit 0 = plan printed (or nothing to do), 1 = a
   precondition failed with an actionable message, 2 is not reachable here.

2. **Read the plan out to the user** before applying. Lines marked `[--force]`
   widen access beyond the one repo — say what else they touch. The script
   spells the blast radius out (`— this also grants it on …`); do not paraphrase
   it away.

3. **Apply** once the user is happy:

   ```bash
   ./enable-semantic-release.py <url>
   ```

   Exit 2 means the plan contains gated steps and needs explicit consent. Do
   **not** re-run with `--force` on your own — go back to the user with what the
   gated step widens, and let them say yes.

   ```bash
   ./enable-semantic-release.py <url> --force   # only after the user agrees
   ```

4. **Hand over the release command.** On success the script prints the `sed …|
   oc create -f -` that starts the Job. Print it; do not run it. The grant and
   the run stay separately auditable. Drop the third `-e` for a dry-run release.
   Once the targets are installed, `just release` is the same Job with the
   preflight in front of it — say so, still do not run it.

## Managed blocks

Both installed blocks obey the same rules, from the same code (`class
Managed`). Only three things differ per block: the comment syntax, the
candidate filenames, and what counts as a clash.

### The just targets

Installed into the git top level of the cwd (`--repo-dir DIR` to aim
elsewhere), into `justfile`, `Justfile` or `.justfile` — whichever is there, a
lower-case `justfile` if none is:

| Target | What it does |
| --- | --- |
| `preflight` | Every precondition, read-only. Also what `release` depends on. |
| `release-dry` | A Job with `--dry-run`: no tag, no commit, no Release. |
| `release` | The real thing. |
| `clean-jobs` | Delete this repo's finished release Jobs, after a prompt. |

Variables are prefixed `sr_`; there is no `set shell` and no `default` recipe,
because the block is *appended* to a file that may already have both and `just`
rejects a duplicate setting. Nothing in the block is machine-specific: the slug,
the manifest and the script are all located at `just` time.

forge-agents runs this block itself — its `Justfile` is a header, a `default`,
and then the fence. Edit `justfile-block.just`, never the installed copy.

### The AGENTS.md how-to

Ten lines: the Job releases the release remote's `main` from the forge — that
is the current branch's tracking remote, `origin` unless the checkout says
otherwise, printed by `just preflight` and overridable with
`just sr_remote=<name> …` — and never sees your
working tree, `just preflight` before `just release`, commits must be
conventional. It points at the just targets; it does not restate them. Edit
`agents-md-block.md`.

**Which file.** `AGENTS.md` first, then `CLAUDE.md`; whichever exists, and
`AGENTS.md` created if neither does.

- only `CLAUDE.md` exists → written to `CLAUDE.md`. That is the file that repo's
  agents actually read.
- both exist and one is a symlink to the other (forge-agents' arrangement) →
  one file, written once.
- both exist as **separate** files → `AGENTS.md` is written and the run prints a
  loud `⚠️ NOTE` that `CLAUDE.md` was not touched. It never writes two files:
  two copies of one instruction set is how they come to disagree, and this tool
  can only keep one of them honest.

### Markers

```
# >>> enable-semantic-release v1.2.3 sha256:6e54f7c098a2 (managed block — do not edit) >>>
<!-- >>> enable-semantic-release v1.2.3 sha256:6e54f7c098a2 (managed block — do not edit) >>> -->
```

Same grammar, wrapped in whatever hides a line from the parser: `#` for a
justfile, an HTML comment for Markdown, where `#` is a heading and not a comment
at all. The closing line repeats both facts:

```
<!-- <<< enable-semantic-release v1.2.3 sha256:6e54f7c098a2 <<< -->
```

Both facts, on both lines, because two different questions get asked later: the
**version** answers "is there a newer block available", the **hash** (sha256 of
the body, first 12) answers "is this still mine to replace". Repeating them on
the closing line makes a half-deleted fence a mismatch rather than a silent
truncation, and the reader regex is built from the writer's format string so the
two cannot drift.

⚠️ This deviates from the Beads `<!-- BEGIN BEADS INTEGRATION v:1 … hash:… -->`
fence already in this repo's AGENTS.md, whose END line is bare. Deliberate: a
bare END makes a deleted END indistinguishable from a deleted block, and this
skill's two fences must be recognisable as the *same* fence across two file
types — matching beads in Markdown would have made the Markdown fence look like
beads' and unlike this skill's justfile fence.

`SKILL_VERSION` in `enable-semantic-release.py` is the *only* place a version is
written. Nobody bumps it by hand: `.releaserc`'s `prepareCmd` seds the release
version in just before tarring the skill, so a released tarball carries its own
number while a forge-agents checkout honestly reads `0.0.0-dev` — which makes
every installed release look newer than the checkout, which it is.

### What it does in each case

| Situation | What happens | Exit |
| --- | --- | --- |
| file absent | creates it containing the block | 0 |
| file there, no block, no clash | appends the block | 0 |
| file there, no block, **clash** | ⚠️ writes nothing, names what it found and where | 1 or 2 |
| block, same version, hash matches | no-op, says so | 0 |
| block, older version, hash matches | replaces it **in place, automatically** — no flag; surrounding content untouched | 0 |
| block, **newer** version than this skill | ⚠️ that would be a downgrade: writes nothing | 2 |
| block, hash does not match its version | ⚠️ you edited it: writes nothing | 2 |
| either of those two with `--force-block` | writes anyway; the older content / your edits are lost | 0 |
| fence broken (one marker, or markers disagreeing) | refuses; repair by hand | 1 |

### Downgrades

An unedited *older* block is replaced with no flag. An unedited *newer* one is
not: that is a downgrade, and the commonest way to reach one is running a
forge-agents checkout — `0.0.0-dev`, which is no release at all and sorts below
every real version — over a repo that installed a released tarball. Install the
newer skill and run that, or `--force-block` to go back deliberately.

Only `MAJOR.MINOR.PATCH` is compared; prerelease and build metadata are dropped,
because `2.0.0-rc.1` ships the same block as `2.0.0`. Whether to *rewrite* is
still exact string equality, so the marker always names the version that wrote
it — a `2.0.0` skill over a `2.0.0-rc.1` block rewrites the marker and nothing
else.

### Clashes

"Something like this is already here, hand-written." The question is the same;
the evidence and the consequence are not.

**justfile — names, and it is fatal (exit 1).** A line scan for recipe, alias
and variable names. `--force-block` does **not** override it: appending would
define `release` twice and `just` would then refuse to parse the file at all, so
a flag would buy a broken justfile. Rename yours and re-run. Over-eager costs a
warning; under-eager costs an unparseable file, so it errs eager. It misses
names pulled in by `import`/`mod`.

**AGENTS.md — content, and it is soft (exit 2).** Markdown has no namespace, so
there is nothing to collide with mechanically. Two signals, outside the managed
block: a heading matching `/releas/i`, or any line naming the machinery (`just
release`, `just preflight`, `release-dry`, `semantic-release`).

⚠️ Signal two is **deliberately over-eager**. forge-agents' own AGENTS.md trips
it on a heading about its bead database, which is a false positive and the right
direction to fail in: a spurious warning costs one `--force-block`, a spurious
append costs a file that silently contradicts itself about how the repo
releases, in the one document agents are told to trust. The known false negative
is a file that explains releasing without naming any tool ("push to main and the
bot does the rest") — nothing short of reading it catches that, and an agent
running this skill is expected to have read it.

## Flags

| Flag | Meaning |
| --- | --- |
| `--dry-run` | plan every part, change nothing. Excludes `--force*`. |
| `--force` | permit grant steps that widen access beyond this repo |
| `--force-block` | permit replacing a hand-edited managed block, or appending past a soft clash |
| `--only PARTS` | comma-separated subset of `grant,justfile,agents-md` |
| `--skip PARTS` | everything except those |
| `--repo-dir DIR` | which repo's files (default: git top level of the cwd) |

`--only`/`--skip` rather than a `--no-X` and an `--X-only` per part: three parts
would have meant six such flags, a fourth part eight. One pair that takes part
names says the same thing and composes — `--only agents-md --force-block` scopes
a force to one file. `just preflight` calls the script with `--only grant`, so it
cannot rewrite the justfile it is running from.

⚠️ `--force` and `--force-block` are separate on purpose and must stay separate:
one consents to widening a bot's access on the forge, the other to losing your
own words in a local file. Folding them would make asking for the second
silently authorise the first.

## Tests

`just test` — stdlib `unittest`, in `tests/`, 71 cases covering both consumers:
round-trip writer→reader across both comment syntaxes and seven version
strings, fence integrity, all seven planner actions, `--dry-run` writing
nothing in every one of them, the splice preserving content on both sides, the
AGENTS.md file-choice matrix, the clash asymmetry, version ordering, and — where
`just` is installed, skipped cleanly otherwise — that generated justfiles
actually parse.

Hermetic: scratch dirs only, no cluster, no network. The grant path is **not**
covered; it needs a live Forgejo and a live Secret, and a mock forge would test
the mock. `tests/` lives outside `skills/`, so it does not ship in the tarball.

⚠️ `preflight` does not run them: `preflight` is part of the shared block, and a
repo that merely installs this skill has no tests to run. In forge-agents, `just
test` is a separate hand-maintained target above the fence.

## Failure modes and what they mean

| What you see | What it is |
| --- | --- |
| `no SSH key` / `no GPG key` | Run `./create-forge-agent.py b4mad-release-agent --overwrite`. A missing GPG key does not fail the push — it lands the release commit **unverified**, silently. |
| `Secret forgejo-agent-… is missing` | The SealedSecret is not in the `b4mad-forgejo` kustomization; see `openshift/README.md`. |
| `has no semantic-release config` | The repo needs a `.releaserc`. Nothing to enable yet. |
| `org '<x>' has no 'release-bots' team` | Copy the team shape from `agentic-forges` using the curl the script prints. It must carry `repo.code=write` — a team that is nominally "write" without that unit still rejects the push. |
| `'<x>' is a personal account` | Forgejo has no teams there. The script offers transfer-to-org (preferred) or a direct collaborator grant, with the cost of each. Present both; the choice is the user's. |
| `enable_push=false` | Branch protection forbids direct pushes outright. The script will not disable a protection — that is a hand edit or a move to PR/AGit flow. |

## Rules

- Never pass `--force` or `--force-block` without the user's explicit go-ahead
  in this session. `--force-block` destroys work someone did by hand; show them
  a diff of the block first if they want one, and scope it with `--only` so it
  cannot reach a part they have not looked at.
- Never edit an installed managed block. Edit `justfile-block.just` or
  `agents-md-block.md` and re-run. An edited block is exactly what the hash
  exists to catch.
- An `agents-md` clash is a prompt to *read the file*, not to force past it.
  If it already says how to release, the right answer is usually to delete the
  hand-written section, not to append a second one.
- Never edit branch protection by hand to work around a refusal. If the script
  declined, it declined for a reason written in the docstring.
- `--dry-run` and `--force` are mutually exclusive; the script rejects the pair.
