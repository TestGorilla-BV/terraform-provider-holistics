package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/TestGorilla-BV/terraform-provider-holistics/internal/provider"
)

var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Run the provider in debug mode (attach to a Terraform CLI for tracing).")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/TestGorilla-BV/holistics",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
