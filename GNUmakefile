default: test

# Unit and emulator-integration tests. Emulator-backed packages skip
# themselves when neither Docker nor SPANNER_EMULATOR_HOST is available.
.PHONY: test
test:
	go test ./... -timeout 15m

# Terraform-driven acceptance tests (internal/provider). Needs a terraform
# binary on PATH (or TF_ACC_TERRAFORM_PATH / TF_ACC_TERRAFORM_VERSION) and a
# Spanner backend: Docker, SPANNER_EMULATOR_HOST, or ALIS_OS_* for live GCP.
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./internal/provider/ -v $(TESTARGS) -timeout 60m

# Everything, acceptance included. Long: also runs the services
# integration suite against the emulator.
.PHONY: testacc-all
testacc-all:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	golangci-lint fmt ./...

.PHONY: docs
docs:
	go generate ./...

# Fails when generated output is stale relative to schemas/examples. Uses
# git status, not git diff: a docs page for a brand-new resource is untracked,
# and git diff cannot see it. examples/ is checked too — go generate runs
# terraform fmt over it, so it is generated output as much as docs/ is.
.PHONY: docs-check
docs-check: docs
	@changed="$$(git status --porcelain -- docs/ examples/)"; \
	if [ -n "$$changed" ]; then \
		echo "Generated output is out of date; run 'make docs' and commit the result:"; \
		echo "$$changed"; \
		exit 1; \
	fi
