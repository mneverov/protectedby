#!/usr/bin/env bash
# Compares BenchmarkRun at a base revision with the working tree and prints benchstat's output and a summary table.
# The current bench_test.go runs against the base's analyzer, so the base doesn't need to have it.
# Runs alternate between base and head to spread machine noise evenly. Each side gets 2*ROUNDS samples.
#
# Usage: scripts/bench-compare.sh [base revision]   (default: origin/main)
# Env:   ROUNDS (default 3), BENCHSTAT (benchstat command), GITHUB_STEP_SUMMARY (the table is appended if set)
set -euo pipefail

base=${1:-origin/main}
rounds=${ROUNDS:-3}
benchstat=${BENCHSTAT:-go run golang.org/x/perf/cmd/benchstat@v0.0.0-20261009192801-be2c69fb417e}
bench=(go test ./protectedby -run '^$' -bench '^BenchmarkRun$' -benchmem -count 2)

if ! git rev-parse --verify --quiet "$base^{commit}" >/dev/null; then
	echo "base revision $base not found, using HEAD~1" >&2
	base=HEAD~1
fi

tmp=$(mktemp -d)
trap 'git worktree remove --force "$tmp/base" >/dev/null 2>&1 || true; rm -rf "$tmp"' EXIT
git worktree add --quiet --detach "$tmp/base" "$base"
cp protectedby/bench_test.go "$tmp/base/protectedby/"

for i in $(seq "$rounds"); do
	echo "round $i/$rounds" >&2
	(cd "$tmp/base" && "${bench[@]}") >>"$tmp/base.txt"
	"${bench[@]}" >>"$tmp/head.txt"
done

echo "base: $(git rev-parse --short "$base"), head: $(git rev-parse --short HEAD)$(git diff --quiet HEAD -- protectedby || echo ' + local changes')"
$benchstat base="$tmp/base.txt" head="$tmp/head.txt"
echo
table=$($benchstat -format csv base="$tmp/base.txt" head="$tmp/head.txt" 2>/dev/null | awk -f scripts/benchtable.awk)
echo "$table"
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
	printf '### Benchmark: %s → %s\n\n%s\n' "$(git rev-parse --short "$base")" "$(git rev-parse --short HEAD)" \
		"$table" >>"$GITHUB_STEP_SUMMARY"
fi
