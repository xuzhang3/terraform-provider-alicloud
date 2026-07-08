package ram

import (
	"context"
	"fmt"
	"log"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// clearAccessKeysAction is a terraform-plugin-framework Action
// (alicloud_ram_user_clear_access_keys) that deletes all access keys of a RAM
// user. Actions are imperative operations invoked from configuration
// (Terraform >= 1.14); unlike resources they do not manage state.

var (
	_ action.Action              = &clearAccessKeysAction{}
	_ action.ActionWithConfigure = &clearAccessKeysAction{}
)

// NewClearAccessKeysAction returns the alicloud_ram_user_clear_access_keys action.
func NewClearAccessKeysAction() action.Action {
	return &clearAccessKeysAction{}
}

type clearAccessKeysAction struct {
	client *connectivity.AliyunClient
}

type clearAccessKeysConfigModel struct {
	UserName types.String `tfsdk:"user_name"`
}

func (a *clearAccessKeysAction) Metadata(_ context.Context, _ action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = "alicloud_ram_user_clear_access_keys"
}

func (a *clearAccessKeysAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{
		Description: "Deletes all access keys of a RAM user.",
		Attributes: map[string]actionschema.Attribute{
			"user_name": actionschema.StringAttribute{
				Required:    true,
				Description: "The name of the RAM user whose access keys will be deleted.",
			},
		},
	}
}

func (a *clearAccessKeysAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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
	a.client = client
}

func (a *clearAccessKeysAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config clearAccessKeysConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	userName := config.UserName.ValueString()

	log.Printf("[DDDDD] RAM user %q has access keys to delete", userName)

	listReq := ram.CreateListAccessKeysRequest()
	listReq.RegionId = a.client.RegionId
	listReq.UserName = userName
	raw, err := a.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.ListAccessKeys(listReq)
	})
	if err != nil {
		resp.Diagnostics.AddError("Error listing access keys for "+userName, err.Error())
		return
	}
	aks, _ := raw.(*ram.ListAccessKeysResponse)
	keys := aks.AccessKeys.AccessKey

	if len(keys) == 0 {
		a.progress(resp, fmt.Sprintf("RAM user %q has no access keys to delete", userName))
		return
	}

	for i, k := range keys {
		a.progress(resp, fmt.Sprintf("Deleting access key %d/%d of RAM user %q", i+1, len(keys), userName))

		delReq := ram.CreateDeleteAccessKeyRequest()
		delReq.RegionId = a.client.RegionId
		delReq.UserName = userName
		delReq.UserAccessKeyId = k.AccessKeyId
		if _, err := a.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
			return c.DeleteAccessKey(delReq)
		}); err != nil && !errorHasPrefix(err, "EntityNotExist") {
			resp.Diagnostics.AddError("Error deleting access key "+k.AccessKeyId, err.Error())
			return
		}
	}
}

func (a *clearAccessKeysAction) progress(resp *action.InvokeResponse, message string) {
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: message})
	}
}
