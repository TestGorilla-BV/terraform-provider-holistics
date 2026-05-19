package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/mockserver"
)

// providerConfig is prepended to every test config block.
const providerConfig = `
provider "holistics" {}
`

// protoV6ProviderFactories serves the provider in-process for tests.
var protoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"holistics": providerserver.NewProtocol6WithError(New("test")()),
}

// withMockServer spins up a mock Holistics API, points the provider at it via
// env vars, and tears it down when the test completes.
func withMockServer(t *testing.T) *mockserver.Server {
	t.Helper()
	srv := mockserver.New()
	t.Cleanup(srv.Close)
	t.Setenv("HOLISTICS_API_KEY", "test-key")
	t.Setenv("HOLISTICS_BASE_URL", srv.BaseURL())
	return srv
}

// runResourceTest is a thin wrapper that fills in the factories field.
func runResourceTest(t *testing.T, steps []resource.TestStep) {
	t.Helper()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps:                    steps,
	})
}
