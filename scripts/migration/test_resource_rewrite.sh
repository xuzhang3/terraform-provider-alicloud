#!/usr/bin/env bash
# Migrate package alicloud test files off terraform-plugin-sdk helper/resource onto
# terraform-plugin-testing/helper/resource.
#
# Why: helper/resource registers a -sweep flag in init, and so does
# terraform-plugin-testing/helper/resource. A test binary linking both dies at startup
# with "flag redefined: sweep". Route 1 (retry_rewrite.sh) moved the non-test files, which
# unblocked subpackage test binaries. This is Route 2: it moves package alicloud's own test
# files, so a framework-hosted test can live in package alicloud too and every package can
# call acctest.ProtoV5ProviderFactories.
#
# A pure import-path swap — no identifier is rewritten. terraform-plugin-testing's
# helper/resource is a drop-in superset of the SDK's for everything these tests use:
#   - all 25 resource.* symbols in use exist there (Test, TestCase, TestStep,
#     ComposeTestCheckFunc, AddTestSweepers, TestMain, ParallelTest, TestCheckFunc,
#     Retry, RetryableError, NonRetryableError, RetryError, StateRefreshFunc, ...);
#   - TestCase and TestStep are strict supersets, no field dropped — the deprecated
#     Providers and ProviderFactories are still there with identical types, and
#     terraform-plugin-testing imports the same terraform-plugin-sdk/v2/helper/schema,
#     so *schema.Provider stays type-identical;
#   - the same three sweeper flags (-sweep, -sweep-allow-failures, -sweep-run) are
#     registered, so the sweeper entrypoint is unaffected.
# Retry helpers therefore stay spelled resource.Retry here, unlike in non-test files where
# Route 1 moved them to retry.Retry. Both are correct: the test binary links
# terraform-plugin-testing, the provider binary links helper/retry.
#
# When: run once to migrate the tree, and re-run after every upstream merge — the merge
# reintroduces helper/resource in the test files it touches, and this script converts
# exactly those. Idempotent: with nothing left to migrate it exits 0 and changes nothing.
#
# Covers both SDK import paths: v1 (terraform-plugin-sdk/helper/resource, what
# upstream/master still uses) and v2 (terraform-plugin-sdk/v2/helper/resource).
#
# Requires: grep, perl, gofmt. gofmt is what re-sorts the import group: the new path sorts
# after terraform-plugin-sdk/v2/helper/schema, so a bare swap leaves the block unsorted and
# trips the fmt check. goimports is not needed — the import set does not change size.
#
# Verify afterwards:
#   go vet ./alicloud/... && go test ./alicloud/ -run XXX_nonexistent   # links the binary
#
# Design: "设计：将 package alicloud 从 helper/resource 迁移走（Route 1）" in the vault
# (tf provider/design/release v2/2026-08-17-helper-retry-migration-design.md).
set -euo pipefail
cd "$(dirname "$0")/../.."

IMPORT_V1='github.com/hashicorp/terraform-plugin-sdk/helper/resource'
IMPORT_V2='github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource'
IMPORT_NEW='github.com/hashicorp/terraform-plugin-testing/helper/resource'

FILES="$(grep -rlF -e "\"$IMPORT_V1\"" -e "\"$IMPORT_V2\"" --include='*_test.go' alicloud || true)"

if [ -z "$FILES" ]; then
    echo "nothing to migrate: no test file imports helper/resource"
    exit 0
fi

echo "files to migrate: $(printf '%s\n' "$FILES" | wc -l | tr -d ' ')"

# Rewrite only the import line, anchored on the full quoted path at line start. Nothing
# else in the file is touched: the selector stays "resource.", so a local variable named
# resource cannot be caught the way it could in Route 1.
printf '%s\n' "$FILES" | while IFS= read -r f; do
    perl -pi -e '
        s{^(\s*)"\Qgithub.com/hashicorp/terraform-plugin-sdk/helper/resource\E"(\s*)$}{$1"github.com/hashicorp/terraform-plugin-testing/helper/resource"$2};
        s{^(\s*)"\Qgithub.com/hashicorp/terraform-plugin-sdk/v2/helper/resource\E"(\s*)$}{$1"github.com/hashicorp/terraform-plugin-testing/helper/resource"$2};
    ' "$f"
done

# Re-sort the import group the swap disturbed. Batched: one huge invocation can die
# silently mid-way; -n 200 keeps every batch observable and failures attributable.
printf '%s\n' "$FILES" | xargs -n 200 gofmt -w

echo "done: verify with 'go vet ./alicloud/...'"
