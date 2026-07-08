// Package fwprovider hosts the terraform-plugin-framework provider for
// terraform-provider-alicloud. In production it runs alongside the existing
// SDKv2 provider behind a single gRPC server via terraform-plugin-mux (see the
// fwprovider/mux package), so new resources and data sources can be authored
// with the framework without rewriting the SDKv2 provider.
//
// This package deliberately does not import the alicloud (SDKv2) package, so it
// stays free of terraform-plugin-sdk/v2/helper/resource. That lets
// terraform-plugin-testing-based tests (e.g. statecheck) use the standalone
// framework provider without the duplicate sweep-flag registration panic.
package fwprovider

import (
	"context"

	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/fwprovider/services/account"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/fwprovider/services/ram"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Ensure the framework provider satisfies the provider interfaces.
var (
	_ provider.Provider                       = &alicloudProvider{}
	_ provider.ProviderWithEphemeralResources = &alicloudProvider{}
	_ provider.ProviderWithListResources      = &alicloudProvider{}
	_ provider.ProviderWithActions            = &alicloudProvider{}
)

// alicloudProvider is the terraform-plugin-framework provider. It has two modes:
//
//   - Muxed (primary != nil): it does not parse credentials itself; the muxed
//     SDKv2 provider does, and exposes the *connectivity.AliyunClient through its
//     Meta(), which this provider reuses. This is the production path.
//   - Standalone (client != nil): it serves the framework resources on their own
//     with a supplied client, used by framework-only acceptance tests.
type alicloudProvider struct {
	primary *sdkschema.Provider        // set in muxed mode
	client  *connectivity.AliyunClient // set in standalone mode
}

// NewFrameworkProvider returns the framework provider bound to the SDKv2 provider
// it is muxed with (production path).
func NewFrameworkProvider(primary *sdkschema.Provider) provider.Provider {
	return &alicloudProvider{primary: primary}
}

// NewStandaloneProvider returns a framework provider that serves the framework
// resources without the mux, using the supplied client. It mirrors AzureRM's
// NewFrameworkV5Provider and is intended for framework-only acceptance tests
// that cannot link the SDKv2 provider.
func NewStandaloneProvider(client *connectivity.AliyunClient) provider.Provider {
	return &alicloudProvider{client: client}
}

func (p *alicloudProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "alicloud"
}

// Schema mirrors the SDKv2 provider schema exactly when muxed (tf5muxserver
// requires both servers to advertise an identical provider schema, derived via
// convertProviderSchema). In standalone mode there is no config to parse, so the
// schema is empty.
func (p *alicloudProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	if p.primary == nil {
		resp.Schema = providerschema.Schema{}
		return
	}
	resp.Schema = convertProviderSchema(p.primary.Schema)
}

// Configure supplies the shared client to resources/data sources/ephemeral
// resources: from the muxed SDKv2 provider's Meta() when muxed, or the injected
// client when standalone.
func (p *alicloudProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var meta any = p.client
	if p.primary != nil {
		meta = p.primary.Meta()
	}
	resp.ResourceData = meta
	resp.DataSourceData = meta
	resp.EphemeralResourceData = meta
	resp.ListResourceData = meta
	resp.ActionData = meta
}

// Resources returns the framework resources. Migrating an existing SDKv2 type
// must also remove it from the SDKv2 maps, since the mux forbids a type served
// by both providers.
func (p *alicloudProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		ram.NewUserResource,
	}
}

func (p *alicloudProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// EphemeralResources returns the framework ephemeral resources.
func (p *alicloudProvider) EphemeralResources(_ context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		account.NewAccountInfoEphemeralResource,
	}
}

// ListResources returns the framework list resources (Terraform >= 1.14
// `terraform query`). Each must match the type name of a managed resource.
func (p *alicloudProvider) ListResources(_ context.Context) []func() list.ListResource {
	return []func() list.ListResource{
		ram.NewUserListResource,
	}
}

// Actions returns the framework actions (Terraform >= 1.14): imperative
// operations invoked from configuration.
func (p *alicloudProvider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		ram.NewClearAccessKeysAction,
	}
}
