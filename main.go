package main

import (
	"context"
	"flag"
	"log"

	"github.com/aliyun/terraform-provider-alicloud/alicloud/fwprovider/mux"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	ctx := context.Background()

	// Serve the SDKv2 provider and the terraform-plugin-framework provider muxed
	// together behind a single protocol v5 gRPC server.
	serverFactory, _, err := mux.ProtoV5ProviderServerFactory(ctx)
	if err != nil {
		log.Fatal(err)
	}

	var serveOpts []tf5server.ServeOpt
	if debug {
		serveOpts = append(serveOpts, tf5server.WithManagedDebug())
	}

	err = tf5server.Serve(
		"registry.terraform.io/aliyun/alicloud",
		serverFactory,
		serveOpts...,
	)
	if err != nil {
		log.Fatal(err)
	}
}
