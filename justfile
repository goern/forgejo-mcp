# forgejo-mcp task recipes.
#
# The canonical build tooling is `make` (see Makefile). This justfile holds
# auxiliary recipes that the Make targets don't cover. Run `just --list`.

# Validate anchored Showboat demos against their specs (see .claude/skills/showboat).
check-demos:
    ./scripts/ci/check-spec-demo-anchors.sh
