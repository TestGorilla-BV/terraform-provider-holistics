# terraform-provider-holistics

A Terraform provider for [Holistics](https://holistics.io), built on the [Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework). Wraps the [Holistics API v2](https://docs.holistics.io/api/v2/).

## Status

`v0.2` covers the Holistics API resources that expose full CRUD plus a handful of read-only data sources.

### Resources

| Resource                       | API endpoints                                |
|--------------------------------|----------------------------------------------|
| `holistics_group`              | `/groups`                                    |
| `holistics_user`               | `/users/invite`, `/users/{id}`, `/users/{id}/restore` |
| `holistics_user_attribute`     | `/user_attributes`                           |
| `holistics_data_schedule`      | `/data_schedules`                            |
| `holistics_data_alert`         | `/data_alerts`                               |
| `holistics_shareable_link`     | `/shareable_links`                           |

### Data sources

| Data source                  | API endpoint                 |
|------------------------------|------------------------------|
| `holistics_user`             | `/users` (filtered by `id`)  |
| `holistics_users`            | `/users` (list, filterable)  |
| `holistics_current_user`     | `/users/me`                  |
| `holistics_dashboard`        | `/dashboards/{id}`           |
| `holistics_data_source`      | `/data_sources/{id}`         |
| `holistics_tags`             | `/tags`                      |

Endpoints that are action-only or upsert-only (e.g. `/data_schedules/submit_execute`, `/users/invite`, `/user_attribute_entries/upsert`) are not modeled — Terraform's CRUD lifecycle doesn't map cleanly to them.

## Provider configuration

```hcl
terraform {
  required_providers {
    holistics = {
      source = "TestGorilla-BV/holistics"
    }
  }
}

provider "holistics" {
  api_key = var.holistics_api_key  # or env HOLISTICS_API_KEY
  region  = "apac"                 # apac | us | eu — default apac
  # base_url = "https://staging.holistics.io/api/v2"  # optional override
}
```

| Argument   | Env var               | Required | Default |
|------------|-----------------------|----------|---------|
| `api_key`  | `HOLISTICS_API_KEY`   | yes      | —       |
| `region`   | `HOLISTICS_REGION`    | no       | `apac`  |
| `base_url` | `HOLISTICS_BASE_URL`  | no       | derived from `region` |

The API key is sent as the `X-Holistics-Key` header.

## Building locally

```sh
go build -o terraform-provider-holistics .
```

To wire the local binary into Terraform for development, drop the following into `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "TestGorilla-BV/holistics" = "/absolute/path/to/this/repo"
  }
  direct {}
}
```

## Example

See [examples/](./examples/).

## Design notes

- **Polymorphic destinations.** Both `holistics_data_schedule` and `holistics_data_alert` accept polymorphic `dest` payloads on the API side. The provider exposes them as separate optional nested attributes (`email_dest`, `slack_dest`, `sftp_dest`, `google_sheet_dest`, `email_subscription_dest`, `webhook_dest`); exactly one must be set.
- **Group user membership.** The API exposes group user assignments via `/groups/{id}/add_user/{user_id}` and `/groups/{id}/remove_user/{user_id}` rather than the create/update body. The provider diffs the desired vs current `user_ids` set and issues the appropriate add/remove calls.
- **User reads.** Neither `/users/{id}` nor `/user_attributes/{id}` expose a `GET`, so the provider paginates the list endpoint and filters client-side.
- **User lifecycle.** Holistics has no synchronous "create user" call — `POST /users/invite` returns an async job, and the user record materialises shortly after. The provider waits up to 15s for the record to appear, then issues a follow-up `PUT` for fields not settable at invite time (`name`, `title`, `job_title`). `DELETE /users/{id}` is a *soft* delete, so the provider transparently calls `POST /users/{id}/restore` if a subsequent apply re-invites the same email — letting `terraform destroy && terraform apply` round-trip cleanly. `POST /users/{id}/resend_invite` and `POST /users/{id}/revoke_authentication_token` aren't surfaced as Terraform actions; setting `allow_authentication_token = false` on update has the same effect as the revoke endpoint.
- **Condition values.** The OpenAPI `ConditionValue` is `anyOf [string, boolean, number]`. The provider keeps `values = list(string)` on the Terraform side but the underlying client coerces strings to the matching JSON type on the wire: `"true"`/`"false"` become booleans, canonical numeric strings (e.g. `"42"`, `"-3.14"`, `"1e3"`) become numbers, everything else stays a string. Edge case: literal strings that look like booleans or numbers can't survive the round-trip — uncommon for filter values but worth knowing.

## Running tests

```sh
make test       # unit tests
make testacc    # acceptance tests against an in-process mock
make docs       # regenerate Markdown docs in docs/
```

The acceptance tests spin up an in-memory mock of the subset of the Holistics API the provider depends on (see [internal/mockserver/](./internal/mockserver/)), so they need no credentials or network access.

## Publishing to the Terraform Registry

The repo publishes to `registry.terraform.io/TestGorilla-BV/holistics`. The pipeline is in [.circleci/config.yml](./.circleci/config.yml); releases are cut by GoReleaser ([.goreleaser.yml](./.goreleaser.yml)).

### One-time setup

1. **Repository.** The GitHub repo must be `github.com/TestGorilla-BV/terraform-provider-holistics` — the name is enforced by the registry.
2. **GPG key.** Generate a release-signing key (recommend a dedicated subkey, not a developer's personal key):
   ```sh
   gpg --quick-generate-key "TestGorilla Terraform Releases <devops@testgorilla.com>" rsa4096 default 0
   gpg --list-secret-keys --keyid-format=long  # note the 40-char fingerprint
   gpg --armor --export <FINGERPRINT>          # public key — paste into the registry UI
   gpg --armor --export-secret-keys <FINGERPRINT> | base64 -w0  # private key for CircleCI
   ```
3. **CircleCI context** named `terraform-registry-release` (shared across all TestGorilla-BV Terraform providers) with four env vars:
   - `GPG_PRIVATE_KEY` — base64-encoded ASCII-armored private key
   - `GPG_FINGERPRINT` — 40-char hex fingerprint
   - `GPG_PASSPHRASE` — key passphrase, or empty string if the key has none. Required (gpg refuses to sign in loopback mode without the flag, even if the value is empty).
   - `GITHUB_TOKEN` — fine-grained PAT with `contents:read+write` on the provider repo(s), used to upload release assets
4. **Terraform Registry.** Sign in at [registry.terraform.io](https://registry.terraform.io) with a GitHub account that has admin on the TestGorilla-BV org, click *Publish → Provider*, select the repo. Add the GPG public key when prompted.

### Cutting a release

```sh
git tag v0.1.0
git push origin v0.1.0
```

CircleCI's `release` workflow runs only on tags matching `v\d+\.\d+\.\d+(-\w+)?`. GoReleaser cross-compiles binaries for linux/darwin/windows/freebsd × amd64/386/arm/arm64, generates `SHA256SUMS`, signs them with the GPG key, and uploads everything plus `terraform-registry-manifest.json` to the GitHub release. The registry polls GitHub and indexes the new version within a few minutes.

### Dry-run

```sh
make release-snapshot       # builds + archives without signing or publishing
ls dist/                    # inspect the artifacts GoReleaser would have uploaded
```
