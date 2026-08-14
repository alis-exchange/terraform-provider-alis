# terraform-provider-alis

Manage Google Cloud Spanner schema — tables, indexes, foreign keys, TTL policies, IAM and roles — declaratively with Terraform.

## About

`alis` is a Terraform provider built with the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework), published at `registry.terraform.io/alis-exchange/alis`. It manages Google Spanner schema objects at fine granularity: each table, index, foreign key, TTL policy, IAM binding, database role, and sequence is its own Terraform resource.

**Resources** (generated docs in [`docs/resources/`](docs/resources)):

| Resource | Docs |
|---|---|
| `alis_google_spanner_table` | [google_spanner_table](docs/resources/google_spanner_table.md) |
| `alis_google_spanner_table_index` | [google_spanner_table_index](docs/resources/google_spanner_table_index.md) |
| `alis_google_spanner_table_foreign_key` | [google_spanner_table_foreign_key](docs/resources/google_spanner_table_foreign_key.md) |
| `alis_google_spanner_table_ttl_policy` | [google_spanner_table_ttl_policy](docs/resources/google_spanner_table_ttl_policy.md) |
| `alis_google_spanner_table_iam_binding` | [google_spanner_table_iam_binding](docs/resources/google_spanner_table_iam_binding.md) |
| `alis_google_spanner_database_role` | [google_spanner_database_role](docs/resources/google_spanner_database_role.md) |
| `alis_google_spanner_database_sequence` | [google_spanner_database_sequence](docs/resources/google_spanner_database_sequence.md) |

**Data sources** (generated docs in [`docs/data-sources/`](docs/data-sources)): `alis_google_spanner_database_roles`, `alis_google_spanner_table_iam_binding`.

Note on PROTO columns: a table column is declared as a protocol buffer type via `proto_package` only. The proto bundle must already exist in the database — the provider does not create bundles.

## Installation

Requirements:

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.8 — required by the provider-defined functions and the oldest version the acceptance suite runs against (see the matrix in [`test.yml`](.github/workflows/test.yml)); raise this line and that matrix together
- [Go](https://go.dev/doc/install) >= 1.26.6 (only to build the provider from source) — the patch floor tracks standard-library security fixes, so it moves when `govulncheck` reports one
- Docker (only for emulator-backed tests)
- `protoc` (only to regenerate the proto test fixture)

Add the provider to your configuration and run `terraform init`:

```terraform
terraform {
  required_providers {
    alis = {
      source  = "alis-exchange/alis"
      version = ">= 1.5.0, < 2.0.0"
    }
  }
}
```

## Usage

```terraform
provider "alis" {
  project = var.GOOGLE_PROJECT
}
```

The provider authenticates with Google Cloud in one of three ways (in order of precedence):

1. `credentials` — a JSON string of Google Cloud credentials.
2. `access_token` — an OAuth2 access token (requires `project`).
3. Neither set — [Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials), e.g. `gcloud auth application-default login` or `GOOGLE_APPLICATION_CREDENTIALS` pointing at a service-account key.

See the [provider docs](docs/index.md) for the full schema, the per-resource docs linked above for arguments and import syntax, and [`examples/`](examples) for runnable configurations.

## Run the provider locally

Build and install the provider binary into `$(go env GOBIN)` (defaults to `~/go/bin`):

```sh
go install .
```

Then point Terraform at that directory with a `dev_overrides` block in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "alis-exchange/alis" = "/Users/<you>/go/bin"
  }
  direct {}
}
```

With dev overrides active, skip `terraform init` and run `terraform plan` / `terraform apply` directly — Terraform prints a "Provider development overrides are in effect" warning to confirm your local build is being used.

### Manual verification fixtures

[`testing/resources/`](testing/resources) contains one directory of real `.tf` configs per resource, plus [`functions/`](testing/resources/functions) for the provider-defined functions; [`.template`](testing/resources/.template) is the starting point for a new one:

1. Copy `.template` to `testing/resources/<resource_name>` and write the resource config.
2. Create `terraform.tfvars` with `GOOGLE_PROJECT`, `SPANNER_INSTANCE`, `SPANNER_DATABASE` (table-scoped resources also take `SPANNER_TABLE`). `terraform.tfvars` and `*.tfstate` are gitignored — they hold real project details.
3. `go install .` and run `terraform plan` in that directory with dev overrides active.

## Run tests

Tests come in four tiers, from hermetic to cloud-backed:

1. **Unit tests** — `go test ./...`. No cloud access needed; pure suites run against fakes in `internal/spanner/conn/connfake`.
2. **Emulator-backed tests** — the `emulator*_test.go` files in `internal/spanner/conn` and `internal/spanner/schema` exercise the real GCP adapter against the Cloud Spanner emulator. With Docker running they auto-start `gcr.io/cloud-spanner-emulator/emulator:latest` via testcontainers-go; without Docker or `SPANNER_EMULATOR_HOST` they skip automatically.
3. **Integration lifecycles** — the testify suite `TestIntegrationSuite` in `internal/spanner/services` runs full create → read → mutate → delete lifecycles for every resource. They resolve their backend emulator-first via `conntest.Target`: an explicit `SPANNER_EMULATOR_HOST`, else a Docker-started emulator, else live Spanner if `GOOGLE_PROJECT`/`SPANNER_INSTANCE` are set; otherwise they skip. Set `SPANNER_LIVE=1` to choose live Spanner even when an emulator is reachable — without it, a running Docker daemon always wins and the live-only tests never execute. Every lifecycle creates and removes its own objects, so a live database is left as found. A few assertions the emulator cannot host (IAM-binding reads, database-role listings) run only against live Spanner.
4. **Acceptance tests** — the `TestAcc*` functions in `internal/provider` drive the real provider through a real `terraform` binary (plan, apply, import, destroy) via [terraform-plugin-testing](https://developer.hashicorp.com/terraform/plugin/testing). They are gated behind `TF_ACC=1` (`make testacc`), resolve their backend exactly like the integration lifecycles (each test gets a fresh throwaway database), and need a `terraform` binary on `PATH` — or set `TF_ACC_TERRAFORM_PATH` to a specific binary, or `TF_ACC_TERRAFORM_VERSION` to auto-install one. The database-role and IAM-binding tests skip on the emulator (their reads need `ListDatabaseRoles` and `INFORMATION_SCHEMA.TABLE_PRIVILEGES`, which it does not implement) and run against live Spanner.

With Docker running and no environment set, `go test ./...` therefore covers everything except the live-only assertions — which is the default posture for CI and automated agents.

Environment variables:

| Variable | Tier | Effect |
|---|---|---|
| `SPANNER_EMULATOR_HOST` | Emulator | Reuse an already-running emulator instead of starting a Docker container |
| `SPANNER_EMULATOR_IMAGE` | Emulator | Override the default emulator image |
| `SPANNER_LIVE` | Integration | Set to `1` to run against live Spanner even when an emulator is reachable |
| `GOOGLE_PROJECT` | Integration | Google Cloud project for live runs |
| `SPANNER_INSTANCE` | Integration | Spanner instance for live runs |
| `SPANNER_DATABASE` | Integration | Existing live database the lifecycles run inside (default `tf-test`) |

Without `SPANNER_LIVE`, the `GOOGLE_PROJECT`/`SPANNER_INSTANCE`/`SPANNER_DATABASE` variables only take effect when no emulator can be reached.

Make targets: `make test` (the default — everything except acceptance), `make testacc` (acceptance tests only), `make testacc-all` (everything including acceptance), `make lint`, `make fmt`, `make docs`, and `make docs-check`. CI runs lint, `make docs-check`, `make test`, and the acceptance suite across several Terraform versions on every pull request (`.github/workflows/test.yml`).

The PROTO-column test fixture `internal/spanner/conn/testdata/tftest.pb` is compiled from `tftest.proto`; regenerate it with the `protoc` command in that file's header comment:

```sh
protoc --descriptor_set_out=tftest.pb --include_imports -I . tftest.proto
```

## Generating documentation

The files in `docs/` are generated by [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs) from the provider schema and `examples/`:

```sh
go generate ./...
```

This also runs `terraform fmt` across `examples/` (see the `go:generate` directives in `main.go`). Edit resource schemas and `templates/`/`examples/`, not `docs/` directly.

## Creating a new release

Releases are cut by pushing a semver tag; GitHub Actions does the rest:

1. Ensure commits follow [Conventional Commits](https://www.conventionalcommits.org/) — the changelog is generated from them.
2. Tag and push: `git tag v1.x.y && git push origin v1.x.y`.
3. On a `v*` tag, [`release.yml`](.github/workflows/release.yml) extracts release notes from `CHANGELOG.md` and runs [GoReleaser](.goreleaser.yml), which builds binaries for linux/darwin/windows/freebsd, signs the checksums with the repo's GPG key, and publishes the GitHub release. [`changelog.yml`](.github/workflows/changelog.yml) regenerates `CHANGELOG.md` with `conventional-changelog` and pushes it back.
4. The Terraform Registry picks the new version up from the GitHub release.

## License

[Apache License 2.0](LICENSE)
