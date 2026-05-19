BINARY := terraform-provider-holistics
TFPLUGINDOCS_VERSION ?= v0.20.0
GORELEASER_VERSION   ?= v2.4.5

.PHONY: build test testacc lint vet docs docs-check release-snapshot tools clean

build:
	go build -o $(BINARY) .

test:
	go test ./... -timeout 5m

testacc:
	TF_ACC=1 go test ./... -timeout 10m

vet:
	go vet ./...

# Regenerate Markdown docs from resource/data source schemas. Output goes to docs/.
docs: tools
	tfplugindocs generate --provider-name holistics

# Fail if regenerated docs differ from the committed copy. Wire into CI to keep
# docs honest. The registry rejects providers whose docs/ tree is missing.
docs-check: docs
	@if [ -n "$$(git status --porcelain docs/)" ]; then \
		echo "docs/ is out of date; run 'make docs' and commit."; \
		git --no-pager diff docs/; \
		exit 1; \
	fi

# Local dry-run of the release pipeline. Useful for validating .goreleaser.yml
# without cutting an actual tag.
release-snapshot: tools
	goreleaser release --snapshot --clean --skip=sign,publish

tools:
	@command -v tfplugindocs >/dev/null 2>&1 || go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION)
	@command -v goreleaser >/dev/null 2>&1 || go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

clean:
	rm -f $(BINARY)
	rm -rf dist/
