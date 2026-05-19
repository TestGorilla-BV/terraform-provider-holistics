package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func alreadyExistsRegexp() *regexp.Regexp {
	return regexp.MustCompile(`User already exists`)
}

func TestAccUserResource_inviteAndUpdate(t *testing.T) {
	withMockServer(t)

	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_user" "alice" {
  email          = "alice@example.com"
  role           = "analyst"
  name           = "Alice"
  invite_message = "Welcome!"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_user.alice", "email", "alice@example.com"),
				resource.TestCheckResourceAttr("holistics_user.alice", "role", "analyst"),
				resource.TestCheckResourceAttr("holistics_user.alice", "name", "Alice"),
				resource.TestCheckResourceAttrSet("holistics_user.alice", "id"),
				resource.TestCheckResourceAttr("holistics_user.alice", "is_activated", "false"),
			),
		},
		{
			// Update role + title + group membership.
			Config: providerConfig + `
resource "holistics_user" "alice" {
  email     = "alice@example.com"
  role      = "admin"
  name      = "Alice Smith"
  title     = "Senior Analyst"
  job_title = "Data Lead"
  group_ids = [42]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_user.alice", "role", "admin"),
				resource.TestCheckResourceAttr("holistics_user.alice", "name", "Alice Smith"),
				resource.TestCheckResourceAttr("holistics_user.alice", "title", "Senior Analyst"),
				resource.TestCheckResourceAttr("holistics_user.alice", "job_title", "Data Lead"),
				resource.TestCheckResourceAttr("holistics_user.alice", "group_ids.#", "1"),
			),
		},
		{
			ResourceName:            "holistics_user.alice",
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{"invite_message"},
		},
		{
			// Import by email — the friendlier path. ImportStateIdFunc is
			// how we tell the framework to pass an email string instead of
			// the integer ID it would otherwise pull from state.
			ResourceName:            "holistics_user.alice",
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateId:           "alice@example.com",
			ImportStateVerifyIgnore: []string{"invite_message"},
		},
	})
}

// Confirms the restore-on-recreate path: invite → soft-delete → re-invite the
// same email transparently restores the previous record instead of failing.
func TestAccUserResource_restoreAfterSoftDelete(t *testing.T) {
	srv := withMockServer(t)
	_ = srv

	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_user" "bob" {
  email = "bob@example.com"
  role  = "user"
}
`,
		},
		{
			// Removing the resource triggers the soft-delete via DELETE.
			Config: providerConfig,
		},
		{
			// Re-declaring with the same email: should restore the
			// soft-deleted record rather than fail with "email already in use".
			Config: providerConfig + `
resource "holistics_user" "bob" {
  email = "bob@example.com"
  role  = "user"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_user.bob", "email", "bob@example.com"),
				resource.TestCheckResourceAttr("holistics_user.bob", "role", "user"),
			),
		},
	})
}

// Confirms collision detection: trying to create a resource for an email that
// already maps to a live (non-deleted) user produces a clear error.
func TestAccUserResource_existingNonDeletedRejected(t *testing.T) {
	withMockServer(t)
	runResourceTest(t, []resource.TestStep{
		// Step 1: create.
		{
			Config: providerConfig + `
resource "holistics_user" "first" {
  email = "carol@example.com"
  role  = "user"
}
`,
		},
		// Step 2: a fresh resource pointing at the same email — Holistics
		// rejects the invite, the provider surfaces "User already exists".
		{
			Config: providerConfig + `
resource "holistics_user" "first" {
  email = "carol@example.com"
  role  = "user"
}

resource "holistics_user" "duplicate" {
  email = "carol@example.com"
  role  = "user"
}
`,
			ExpectError: alreadyExistsRegexp(),
		},
	})
}
