# scripts/

Guidance for contributors editing shell scripts under `scripts/`.

## Runtime Portability

- Do not assume non-default tools (for example `rg`, `fd`, `jq`, `yq`) are installed unless the workflow step installs them first.
- Prefer portable primitives (`find`, `grep`, `sed`, `awk`, `perl`, `xargs`).
- If a script requires a non-default binary, add an explicit install step in the owning workflow before invoking the script.

## Failure Conventions

- Keep existing script exit-code semantics where present:
  - `0` success
  - `1` infra/setup error
  - `2` contract/assertion violation
