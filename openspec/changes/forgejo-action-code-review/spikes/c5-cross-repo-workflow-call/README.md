# C5 spike — cross-repo `workflow_call` on Forgejo Actions

## Question

Does Codeberg's Forgejo (currently v11.x) support cross-repo `workflow_call`
with `uses: <owner>/<repo>/.forgejo/workflows/<file>.yml@<tag>` syntax, the
way GitHub Actions does? Specifically:

1. Does Forgejo resolve the called workflow at the pinned tag?
2. Does `secrets: inherit` (or explicit `secrets:` mapping) propagate caller secrets to the called workflow?
3. Does the called workflow receive the caller's `github.event` context (so triggers like `pull_request` see PR data)?

## Method

1. Create library repo on Codeberg, e.g. `goern/c5-spike-lib`.
2. Push `spike-lib/.forgejo/workflows/reusable.yml` into it.
3. Create a tag `v0.0.1` on the library repo's default branch.
4. Set repo secret `SPIKE_SECRET` on `goern/c5-spike-lib` (any throwaway value, e.g. `library-side-secret`).
5. Create consumer repo on Codeberg, e.g. `goern/c5-spike-consumer`.
6. Push `spike-consumer/.forgejo/workflows/caller.yml` into it (the file references `goern/c5-spike-lib@v0.0.1`; edit the `uses:` line to match the library you created).
7. Set repo secret `SPIKE_SECRET` on `goern/c5-spike-consumer` (any throwaway value, e.g. `consumer-side-secret`).
8. Trigger the caller in **two** ways and observe behavior:
   - **Push trigger:** push a commit to the consumer's main branch. Observe whether the caller workflow runs and reaches the library.
   - **PR trigger:** open a PR on the consumer repo. Observe whether the called workflow sees the PR event context.
9. Record what the called workflow prints for:
   - `inputs.who` (should be `"consumer"`)
   - `github.event_name`, `github.repository`, `github.ref`
   - PR number and base ref (PR trigger only)
   - `SPIKE_SECRET` length

## Recorded outcomes

| Outcome | Implication for design D9 |
|---------|---------------------------|
| All three properties work (resolution, secrets, event context) | Keep D9 Path B as recommended; remove "pending C5 spike" markers in design.md D9 and spec.md scenarios |
| Resolution works but `secrets: inherit` (or mapping) does not propagate | Document that consumers must duplicate secrets into the library repo, or fall back to copy-paste (Path A) |
| Resolution works but event context is **caller's repo** not the consumer's PR | Document that fork-PR review logic must be careful about which `github.repository` is in scope; this may break the action's PR-number passing |
| Resolution fails (Forgejo does not support cross-repo `uses:`) | Collapse D9 to copy-paste only (Path A); update spec scenarios |

## State that will be created

- Two repos under `goern/` (recommend deleting after spike)
- One tag on the library repo
- A handful of workflow run records on both repos
- Two repo secrets (throwaway values — do not reuse production secrets)

## Time budget

~30 minutes total: scaffold (10), trigger (5), observe + record (15).

## What I cannot do from this session

- Setting repo secrets via API requires user authentication; user must do
  this manually via Codeberg web UI or via `forgejo-mcp` (if it exposes a
  secret-create tool — check `mcp__codeberg__*` tool list).
