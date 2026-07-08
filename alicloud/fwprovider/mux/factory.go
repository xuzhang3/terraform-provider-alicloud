// Package mux wires the SDKv2 provider and the terraform-plugin-framework
// provider together behind a single protocol v5 gRPC server via
// terraform-plugin-mux. It is separate from the fwprovider package so that
// fwprovider (and framework-only tests) stay free of the alicloud/SDKv2 import
// chain.
package mux

import (
	"context"

	"github.com/aliyun/terraform-provider-alicloud/alicloud"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/fwprovider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-mux/tf5muxserver"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ProtoV5ProviderServerFactory returns a muxed terraform-plugin-go protocol v5
// provider server factory combining the existing SDKv2 provider with the
// terraform-plugin-framework provider. It mirrors the pattern used by
// terraform-provider-aws and terraform-provider-azurerm.
//
// The SDKv2 provider is the primary: it parses credentials and builds the
// shared *connectivity.AliyunClient, which the framework provider reuses via
// primary.Meta(). The primary *schema.Provider is also returned for use in
// tests.
func ProtoV5ProviderServerFactory(ctx context.Context) (func() tfprotov5.ProviderServer, *sdkschema.Provider, error) {
	primary := alicloud.Provider()

	servers := []func() tfprotov5.ProviderServer{
		primary.GRPCProvider,
		providerserver.NewProtocol5(fwprovider.NewFrameworkProvider(primary)),
	}

	muxServer, err := tf5muxserver.NewMuxServer(ctx, servers...)
	if err != nil {
		return nil, nil, err
	}

	return muxServer.ProviderServer, primary, nil
}
