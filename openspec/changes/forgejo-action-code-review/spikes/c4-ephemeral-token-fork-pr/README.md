# C4 spike — ephemeral `GITEA_TOKEN` + `create_pull_review` on fork PR

## Question

Does `${{ secrets.GITEA_TOKEN }}` with `permissions: pull-requests: write` honor
`create_pull_review` on a **fork PR** under `pull_request_target` trigger on
Codeberg Forgejo (currently v11.x)?

## Method

1. Create target test repo on Codeberg, e.g. `goern/c4-spike-target`.
2. Install `workflow.yml` into the target repo at `.forgejo/workflows/c4.yml`.
3. Push a base commit to the target repo's default branch.
4. From a **different owner** (org or second account), fork
   `goern/c4-spike-target`. The fork must be on a different owner because
   Forgejo forbids a user from forking their own repo into the same account.
5. Make a trivial change in the fork on a new branch. Push.
6. Open a PR from the fork branch back to the target's default branch.
7. Observe the workflow run on Codeberg Actions UI:
   - HTTP status of the `create_pull_review` call
   - Whether the review appears on the PR
8. Repeat the test by also opening a same-repo PR (different branch on the
   target itself, not a fork) so we can compare fork vs. same-repo behavior.

## Recorded outcomes

| Outcome | Implication for design D7 |
|---------|---------------------------|
| Ephemeral token works on **both** same-repo and fork PRs | Keep default ephemeral; remove "stalemate pending C4" markers in design.md D7 and spec.md scenarios |
| Ephemeral token works on same-repo only; rejected on fork | Split spec scenarios; default same-repo to ephemeral, fork to PAT-required input; document scope |
| Ephemeral token rejected on both | Flip default to PAT-required; ephemeral path removed; document PAT scope; revisit whether fork-PR review is feasible at all without bot accounts |

## State that will be created

- One target repo under `goern/` (recommend deleting after spike)
- One forked repo under a different owner (recommend deleting after spike)
- One PR + one comment review on the target repo

## Time budget

~30 minutes total: scaffold (10), trigger (5), observe + record (15).

## What I cannot do from this session

- Forking from a different account requires that second account's credentials.
  The user must run step 4 manually unless we can use a Codeberg org under
  `goern/` ownership to fork into (org-forks are allowed on Forgejo).
