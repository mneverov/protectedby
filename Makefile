BASE ?= origin/main

.PHONY: build test bench bench-compare

build:
	go build ./...

test:
	go test ./...

# Runs the benchmarks on the working tree.
bench:
	go test ./protectedby -run '^$$' -bench . -benchmem

# Compares the benchmarks with BASE and prints benchstat's output and a summary table. See scripts/bench-compare.sh.
bench-compare:
	scripts/bench-compare.sh $(BASE)
