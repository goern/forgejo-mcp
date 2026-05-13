# Feasibility spikes for forgejo-action-code-review

Battle-test 2026-05-12 surfaced two runtime questions about Forgejo
behavior that no amount of documentation can answer. Each spike is short
(~30 min), stubbed (no Claude / Anthropic spend), and produces a clear
yes/no/partial result that drives a specific design decision.

| Spike | Question | Drives decision |
|-------|----------|-----------------|
| [C4](./c4-ephemeral-token-fork-pr/) | Does `${{ secrets.GITEA_TOKEN }}` with `permissions: pull-requests: write` honor `create_pull_review` on a **fork PR**? | design.md D7 default token choice |
| [C5](./c5-cross-repo-workflow-call/) | Does Forgejo support cross-repo `workflow_call` with `uses: <owner>/<repo>/...yml@<tag>`, including `secrets: inherit` and event context? | design.md D9 distribution path |

## Execution order

Run C4 and C5 in parallel — they share no state. Each spike has its own
README with method, recorded outcomes, and time budget.

## Cleanup checklist

After both spikes finish, delete the test repos, forks, and tags so we
do not leave junk on Codeberg:

- C4: `goern/c4-spike-target` and the fork (under a different owner)
- C5: `goern/c5-spike-lib`, `goern/c5-spike-consumer`, and the `v0.0.1` tag
