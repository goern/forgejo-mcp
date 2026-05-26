#!/usr/bin/env bash
# Verify that the locally built release-tools image contains all expected tools.
# Used both locally and by the PR build pipeline.
# Requires: podman
set -euo pipefail

RELEASE_TOOLS_IMAGE="${RELEASE_TOOLS_IMAGE:-localhost/release-tools:dev}"

echo "Verifying tools in ${RELEASE_TOOLS_IMAGE}"

FAIL=0

run_check() {
    local label="$1"
    shift
    echo -n "  ${label}: "
    if output=$(podman run --rm "${RELEASE_TOOLS_IMAGE}" sh -c "$*" 2>&1); then
        echo "OK — $(echo "${output}" | head -1)"
    else
        echo "FAIL"
        echo "    output: ${output}"
        FAIL=1
    fi
}

# Each bundled tool must report a version
run_check "go"           "go version"
run_check "syft"         "syft version"
run_check "goreleaser"   "goreleaser --version"
run_check "cosign"       "cosign version"
run_check "govulncheck"  "govulncheck -version"
run_check "jq"           "jq --version"
run_check "curl"         "curl --version"
run_check "node"         "node --version"
run_check "npm"          "npm --version"
run_check "openspec"     "openspec --version"

# Shell must work (Tekton script: blocks require /bin/sh)
run_check "shell"        "/bin/sh -c 'echo ok'"

# mcpb must resolve without network (npm cache prewarmed in build stage)
echo -n "  mcpb (offline npx): "
if output=$(podman run --rm --network=none "${RELEASE_TOOLS_IMAGE}" \
    sh -c 'npx -y @anthropic-ai/mcpb --version 2>&1 || mcpb --version 2>&1' 2>&1); then
    echo "OK — $(echo "${output}" | head -1)"
else
    echo "FAIL (offline mcpb resolution failed)"
    echo "    output: ${output}"
    FAIL=1
fi

if [[ ${FAIL} -ne 0 ]]; then
    echo ""
    echo "VERIFICATION FAILED — one or more tool checks failed. See output above."
    exit 1
fi

echo ""
echo "All tool checks passed for ${RELEASE_TOOLS_IMAGE}"
