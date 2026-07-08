// Package account contains terraform-plugin-framework resources, data sources
// and ephemeral resources related to the Alibaba Cloud account of the provider
// caller.
package account

import (
	"context"
	"fmt"

	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// accountInfoEphemeralResource exposes the caller's account identity as an
// ephemeral resource: the values are produced on Open and never persisted to
// state. It reuses the shared *connectivity.AliyunClient via Configure.

var (
	_ ephemeral.EphemeralResource              = &accountInfoEphemeralResource{}
	_ ephemeral.EphemeralResourceWithConfigure = &accountInfoEphemeralResource{}
)

// NewAccountInfoEphemeralResource returns the alicloud_account_info ephemeral
// resource.
func NewAccountInfoEphemeralResource() ephemeral.EphemeralResource {
	return &accountInfoEphemeralResource{}
}

type accountInfoEphemeralResource struct {
	client *connectivity.AliyunClient
}

type accountInfoModel struct {
	ID        types.String `tfsdk:"id"`
	AccountId types.String `tfsdk:"account_id"`
	RegionId  types.String `tfsdk:"region_id"`
}

func (r *accountInfoEphemeralResource) Metadata(_ context.Context, _ ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = "alicloud_account_info"
}

func (r *accountInfoEphemeralResource) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides identity information about the Alibaba Cloud account used by the provider, as an ephemeral resource (not persisted to state).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The account ID (same as account_id).",
			},
			"account_id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the Alibaba Cloud account of the caller.",
			},
			"region_id": schema.StringAttribute{
				Computed:    true,
				Description: "The region ID configured for the provider.",
			},
		},
	}
}

func (r *accountInfoEphemeralResource) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*connectivity.AliyunClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data",
			fmt.Sprintf("Expected *connectivity.AliyunClient, got %T. This is a bug in the provider.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *accountInfoEphemeralResource) Open(ctx context.Context, _ ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	accountId, err := r.client.AccountId()
	if err != nil {
		resp.Diagnostics.AddError("Unable to determine account id", err.Error())
		return
	}

	result := accountInfoModel{
		ID:        types.StringValue(accountId),
		AccountId: types.StringValue(accountId),
		RegionId:  types.StringValue(r.client.RegionId),
	}

	resp.Diagnostics.Append(resp.Result.Set(ctx, &result)...)
}
