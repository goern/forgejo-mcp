# Retrofit Mode

Retrofit mode rewrites an existing demo that does not yet use anchors into
the anchored shape. Use it when:
- A spec gains `<!-- demos-anchored: true -->` and the sibling demo already
  exists in the old AC-based or freeform shape.
- The caller invokes showboat with an explicit "retrofit" instruction.

Anchor shape and evidence-kind rules live in `anchored-mode.md` — read that
first. Antipattern checklist lives in `authoring-smells.md`.

## Retrofit Procedure

1. **Read the spec** — extract every `#### Scenario:` heading and its
   WHEN/THEN/AND bullets. Compute the slug for each.

2. **Read the existing demo** — identify what content maps to each scenario.
   The existing structure may be `R<n>`, `AC<n>`, or prose headings. Match
   by content similarity, not by position.

   **H2 dedup rule:** If the old demo has an `## AC<n> — <heading>` H2 that
   shadows a quoted `#### Scenario: <same heading>` H4 immediately below it,
   **drop the H2**. Keep only the `#### Scenario:` quote inside the anchor
   block. Rationale: the H2 is navigational scaffolding for the old AC-based
   format; the anchored `#### Scenario:` heading is the canonical proof text
   and already provides the same orientation.

   Before retrofit:
   ```markdown
   ## AC1 — Config, state, and cache have distinct homes

   #### Scenario: Config, state, and cache have distinct homes
   - **WHEN** ...
   ```

   After retrofit:
   ```markdown
   <!-- spec-scenario: spellkave-cli#config-state-and-cache-have-distinct-homes -->
   **Proves:** [spec.md → Scenario: Config, state, and cache have distinct homes](./spec.md#scenario-config-state-and-cache-have-distinct-homes)

   #### Scenario: Config, state, and cache have distinct homes
   - **WHEN** ...
   ```

   **AC numbering:** Drop `AC<n>` numbering entirely on retrofit. Gaps
   appear naturally when scenarios are inserted mid-sequence; numbered
   headings create false continuity. Use the scenario heading as the
   sole identifier.

3. **Rewrite stale provenance links** — if the old demo has a
   `[openspec/changes/<old-name>]` link that no longer matches the
   merged capability, replace the whole provenance block with a single
   one-liner:

   ```
   *Originally captured for `<old-cap>` / PR #N; merged into `<new-cap>` on YYYY-MM-DD.*
   ```

   Do not try to keep the old multi-field header block alive — it rots.

4. **Surface test candidates** — for each scenario slug, grep the following
   paths for test function names or filter strings that resemble the slug:
   - `server/tests/`
   - `client/tests/`
   - `packages/*/test` (if present)

   A candidate matches when its function name, test name, or `--exact` filter
   contains any substring of the scenario slug (at least one full slug token).
   Report candidates to the caller before writing.

   If a 1:1 candidate exists: propose `evidence-kind: test-invocation`.
   If no 1:1 exists: propose `evidence-kind: unit-test-output` with a
   placeholder comment naming which assertions prove the scenario.
   If the scenario is CLI-observable: propose `evidence-kind: showboat-cli`.

5. **Write the anchored demo** — for each scenario in spec order:
   - Emit the machine anchor, human anchor, quoted scenario block, then the
     evidence block chosen in step 4.
   - Preserve any real captured output from the old demo when it maps cleanly
     to the scenario.
   - Where output is stale or missing, use a placeholder:
     ```bash
     # TODO: re-run against live instance to capture real output
     $ <command>
     <output pending>
     ```

6. **Record the evidence-kind choice inline** — even for `showboat-cli` proofs,
   add `<!-- evidence-kind: showboat-cli -->` before the fenced block so the
   checker and future authors see the intent.

7. **Note test-split decisions** — when a `unit-test-output` placeholder is
   chosen instead of `test-invocation`, add an inline comment explaining why
   (`<!-- retrofitted: no 1:1 test found; unit-test-output fallback -->`).
   This is the data the Tier-C pilot is meant to surface.

8. **Verify.** Run `just check-demos`; fix any reported errors before committing.
