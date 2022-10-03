.SHELLFLAGS := -eu -c
.ONESHELL:			# all things in a rule run in one shell, rather than shell per line
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

.PHONY: build
build: one.js two.js

one.js: one.ts
	@echo "Building one.js"
	@sleep 3s
	@touch "$@"

two.js: two.ts three.js
	@echo "Building two.js"
	@sleep 1s
	@touch "$@"

three.js: three.ts
	@echo "Building three.js"
	@sleep 5s
	@touch "$@"