package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
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

// Regression test: simulates the post-import scenario. After a user is
// created with several Computed fields populated (name, title, etc.), changing
// the role-only in config should produce a plan that touches *only* role —
// not flag all Computed fields as "(known after apply)".
//
// This catches the common Plugin Framework gotcha where Computed and
// Optional+Computed attributes need explicit `UseStateForUnknown` plan
// modifiers to remain stable across updates.
func TestAccUserResource_partialUpdateNoSpuriousDrift(t *testing.T) {
	withMockServer(t)

	runResourceTest(t, []resource.TestStep{
		{
			// Step 1: full config — populates state.
			Config: providerConfig + `
resource "holistics_user" "natalie" {
  email     = "natalie@example.com"
  role      = "analyst"
  name      = "Natalie"
  title     = "Analyst"
  job_title = "Reporting"
}
`,
		},
		{
			// Step 2: only role changes. All other fields are omitted from
			// config. With UseStateForUnknown plan modifiers, the plan must
			// show ONLY role + group_ids drift, not name/title/job_title/etc.
			Config: providerConfig + `
resource "holistics_user" "natalie" {
  email = "natalie@example.com"
  role  = "explorer"
}
`,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("holistics_user.natalie", plancheck.ResourceActionUpdate),
					// These attributes were populated in step 1 and aren't in
					// step 2's config. Without UseStateForUnknown plan
					// modifiers, the plan would mark them "(known after
					// apply)" — these assertions catch that regression by
					// requiring the planned value to equal the prior state
					// value.
					plancheck.ExpectKnownValue("holistics_user.natalie", tfjsonpath.New("name"), knownvalue.StringExact("Natalie")),
					plancheck.ExpectKnownValue("holistics_user.natalie", tfjsonpath.New("title"), knownvalue.StringExact("Analyst")),
					plancheck.ExpectKnownValue("holistics_user.natalie", tfjsonpath.New("job_title"), knownvalue.StringExact("Reporting")),
				},
			},
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_user.natalie", "role", "explorer"),
				// Crucially: these stay at their step-1 values, not "".
				resource.TestCheckResourceAttr("holistics_user.natalie", "name", "Natalie"),
				resource.TestCheckResourceAttr("holistics_user.natalie", "title", "Analyst"),
				resource.TestCheckResourceAttr("holistics_user.natalie", "job_title", "Reporting"),
			),
		},
	})
}

// Regression test: the Holistics /users list endpoint returns a subset of the
// fields PUT /users/{id} returns — `title` and `job_title` in particular are
// populated by PUT but omitted by list. Read uses the list, so state ends up
// with title=null; Update used to write the PUT response, which has title="X"
// for users whose title was set server-side (e.g. via the Holistics UI before
// the user was managed by Terraform).
//
// Without the "write plan to state on Update" fix, the planmodifier pins
// title=null in the plan but the PUT response gives the apply a different
// value, producing "Provider produced inconsistent result after apply".
func TestAccUserResource_listTruncatedUpdateConsistent(t *testing.T) {
	srv := withMockServer(t)
	srv.TruncateListUserDetails = true

	runResourceTest(t, []resource.TestStep{
		{
			// Step 1: create the user with no title via config. After this,
			// the mock has title=nil for the user. State.title is null.
			Config: providerConfig + `
resource "holistics_user" "diana" {
  email = "diana@example.com"
  role  = "analyst"
}
`,
		},
		{
			// Step 2: between steps, simulate the user having had a title
			// assigned via the Holistics UI. The /users list still truncates
			// it, so the refresh inside this step's plan keeps state.title
			// null. Then change only `role` in config — plan.title=null via
			// UseStateForUnknown. PUT response will include title="Set in
			// UI"; without the fix the apply check rejects that.
			PreConfig: func() {
				if err := srv.SetUserFieldByEmail("diana@example.com", "title", "Set in UI"); err != nil {
					t.Fatalf("preconfig: %v", err)
				}
			},
			Config: providerConfig + `
resource "holistics_user" "diana" {
  email = "diana@example.com"
  role  = "explorer"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_user.diana", "role", "explorer"),
				// title stays null in state — the list endpoint truncates
				// it. The server-side value ("Set in UI") is preserved
				// because our Update PUT body never sent a `title` field.
				resource.TestCheckNoResourceAttr("holistics_user.diana", "title"),
			),
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
