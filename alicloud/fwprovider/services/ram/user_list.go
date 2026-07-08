package ram

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// userListResource is the terraform-plugin-framework List resource for
// alicloud_ram_user_v2. It enumerates existing RAM users via the ListUsers API
// so they can be discovered with `terraform query` (Terraform >= 1.14). Its type
// name must match the managed resource it lists.
//
// This is a "pure" framework list resource: because alicloud_ram_user_v2 is
// framework-native, the results reuse the same identity (userIdentityModel) and
// state model (userModel) directly, with no SDKv2 bridge (unlike AWS/AzureRM,
// whose list resources adapt their SDK-based resources).

var (
	_ list.ListResource              = &userListResource{}
	_ list.ListResourceWithConfigure = &userListResource{}
)

// NewUserListResource returns the alicloud_ram_user_v2 List resource.
func NewUserListResource() list.ListResource {
	return &userListResource{}
}

type userListResource struct {
	client *connectivity.AliyunClient
}

// userListConfigModel is the query configuration for listing users.
type userListConfigModel struct {
	NameRegex types.String `tfsdk:"name_regex"`
}

func (l *userListResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "alicloud_ram_user_v2"
}

func (l *userListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	l.client = client
}

func (l *userListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Description: "Lists RAM users. Optionally filter by a name regular expression.",
		Attributes: map[string]listschema.Attribute{
			"name_regex": listschema.StringAttribute{
				Optional:    true,
				Description: "A regex string used to filter results by user name.",
			},
		},
	}
}

func (l *userListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var config userListConfigModel
	if req.Config.Raw.IsKnown() && !req.Config.Raw.IsNull() {
		if diags := req.Config.Get(ctx, &config); diags.HasError() {
			stream.Results = list.ListResultsStreamDiagnostics(diags)
			return
		}
	}

	var nameRegex *regexp.Regexp
	if isKnown(config.NameRegex) {
		re, err := regexp.Compile(config.NameRegex.ValueString())
		if err != nil {
			stream.Results = list.ListResultsStreamDiagnostics(diag.Diagnostics{
				diag.NewAttributeErrorDiagnostic(path.Root("name_regex"), "Invalid name_regex", err.Error()),
			})
			return
		}
		nameRegex = re
	}

	stream.Results = func(yield func(list.ListResult) bool) {
		listReq := ram.CreateListUsersRequest()
		listReq.RegionId = l.client.RegionId
		listReq.MaxItems = requests.NewInteger(1000)

		for {
			raw, err := l.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
				return c.ListUsers(listReq)
			})
			if err != nil {
				yield(list.ListResult{Diagnostics: diag.Diagnostics{
					diag.NewErrorDiagnostic("Error listing RAM users", err.Error()),
				}})
				return
			}
			listResp, _ := raw.(*ram.ListUsersResponse)

			for _, u := range listResp.Users.User {
				if nameRegex != nil && !nameRegex.MatchString(u.UserName) {
					continue
				}

				result := req.NewListResult(ctx)
				result.DisplayName = u.UserName

				result.Diagnostics.Append(result.Identity.Set(ctx, userIdentityModel{
					ID: types.StringValue(u.UserId),
				})...)

				if req.IncludeResource {
					result.Diagnostics.Append(result.Resource.Set(ctx, userModel{
						ID:          types.StringValue(u.UserId),
						Name:        types.StringValue(u.UserName),
						DisplayName: types.StringValue(u.DisplayName),
						Mobile:      types.StringValue(u.MobilePhone),
						Email:       types.StringValue(u.Email),
						Comments:    types.StringValue(u.Comments),
						Force:       types.BoolNull(),
					})...)
				}

				if !yield(result) {
					return
				}
			}

			if !listResp.IsTruncated {
				break
			}
			listReq.Marker = listResp.Marker
		}
	}
}
