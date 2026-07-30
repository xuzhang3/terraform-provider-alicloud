// Package ram contains the terraform-plugin-framework resources and data
// sources for the RAM (Resource Access Management) product.
package ram

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdkerrors "github.com/aliyun/alibaba-cloud-sdk-go/sdk/errors"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// userResource is a terraform-plugin-framework re-implementation of the SDKv2
// alicloud_ram_user resource, registered under a new type name
// (alicloud_ram_user_v2) so it can be served alongside the SDKv2 provider
// through the mux (a given type may be served by only one server). It uses the
// exported client helpers (WithRamClient, RpcPost).

var (
	_ resource.Resource                = &userResource{}
	_ resource.ResourceWithConfigure   = &userResource{}
	_ resource.ResourceWithImportState = &userResource{}
	_ resource.ResourceWithIdentity    = &userResource{}
	_ resource.ResourceWithModifyPlan  = &userResource{}
)

// userIdentityModel is the resource identity: a single stable id. Identity is
// required for the framework List resource (see user_list.go).
type userIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

// NewUserResource returns the alicloud_ram_user_v2 framework resource.
func NewUserResource() resource.Resource {
	return &userResource{}
}

type userResource struct {
	client *connectivity.AliyunClient
}

type userModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	DisplayName types.String `tfsdk:"display_name"`
	Mobile      types.String `tfsdk:"mobile"`
	Email       types.String `tfsdk:"email"`
	Comments    types.String `tfsdk:"comments"`
	Force       types.Bool   `tfsdk:"force"`
	AccessKey   types.String `tfsdk:"access_key"`
}

func (r *userResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "alicloud_ram_user_v2"
}

func (r *userResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The RAM user ID.",
			},
		},
	}
}

// ANSI escapes used to colour the rename warning red. Terraform already renders
// warnings in yellow; these are passed through its diagnostic formatter to the
// terminal untouched, which also means -no-color does not strip them and they
// appear literally in -json output and in non-TTY CI logs.
const (
	ansiRed   = "\033[31m"
	ansiReset = "\033[0m"
)

// redLines wraps each line of s in the red escape. Terraform word-wraps a
// diagnostic detail and re-emits its own colour reset between the resulting
// lines, so one leading escape would only tint the first line. Callers should
// hard-wrap s well inside the terminal width: any line Terraform still has to
// re-wrap loses its colour after the break.
func redLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = ansiRed + line + ansiReset
	}
	return strings.Join(lines, "\n")
}

// ModifyPlan is a resource-level plan modifier. It runs on every plan and emits
// two warnings for an existing user:
//   - on a destroy plan (Plan is null) when force is not set, because the delete
//     will fail if the user still has access keys, policies, group memberships, a
//     login profile or an MFA device;
//   - on an update plan when name changes, because the rename happens in place
//     and anything referencing the old user name keeps pointing at it.
func (r *userResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Create has a null State: nothing to compare against.
	if req.State.Raw.IsNull() {
		return
	}

	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only a destroy plan has a null Plan.
	if req.Plan.Raw.IsNull() {
		if !state.Force.ValueBool() {
			resp.Diagnostics.AddWarning(
				"Deleting alicloud_ram_user_v2 without force",
				fmt.Sprintf("RAM user %q is planned for deletion but force is not set to true. The delete will "+
					"fail if the user still has access keys, policies, group memberships, a login profile or an "+
					"MFA device. Set force = true to remove them automatically.", state.Name.ValueString()),
			)
		}
		return
	}

	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// custom warning message with red alert
	if isKnown(plan.Name) && !plan.Name.Equal(state.Name) {
		resp.Diagnostics.AddWarning(
			"Renaming alicloud_ram_user_v2 in place",
			redLines(fmt.Sprintf("This RAM user will be renamed in place:\n"+
				"  from %[1]q\n"+
				"  to   %[2]q\n"+
				"The user ID, access keys, policies and group memberships are kept.\n"+
				"The old name stops working immediately: the console logon name\n"+
				"changes, and anything referring to the user by name (RAM policy\n"+
				"documents, resources configured with user_name, external scripts)\n"+
				"must be updated separately.",
				state.Name.ValueString(), plan.Name.ValueString())),
		)
	}
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a RAM User resource (terraform-plugin-framework implementation).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The RAM user ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The user name.",
			},
			"display_name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The display name of the user. Defaults to the user name when not set.",
				PlanModifiers: []planmodifier.String{
					//defaultDisplayNameToName(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mobile": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "The mobile phone number of the user.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "The email of the user.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"comments": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Comments about the user. Up to 128 characters.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"force": schema.BoolAttribute{
				Optional:    true,
				Description: "When set to true, deleting the user also removes its access keys, policies, group memberships, login profile and MFA device.",
			},
			"access_key": schema.StringAttribute{
				Optional:    true,
				WriteOnly:   true,
				Description: "A write-only access key value for the user; supplied in config but never stored in state.",
			},
		},
	}
}

// displayNameDefaultModifier defaults display_name to the user name when the
// user has not configured a display_name. It is implemented as a plan modifier
// (rather than a schema Default) because the default value is derived from
// another attribute (name), which Default functions cannot access.
type displayNameDefaultModifier struct{}

func defaultDisplayNameToName() planmodifier.String {
	return displayNameDefaultModifier{}
}

func (m displayNameDefaultModifier) Description(_ context.Context) string {
	return "Defaults display_name to the user name when not configured."
}

func (m displayNameDefaultModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m displayNameDefaultModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// The user explicitly set a value: keep it.
	if !req.ConfigValue.IsNull() {
		return
	}
	// There is already a value in prior state (update): keep it; UseStateForUnknown
	// preserves it.
	//if !req.StateValue.IsNull() {
	//	return
	//}
	// Create without a configured display_name: default it to the user name so
	// the planned value is known and matches what Create sends to the API.
	var name types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() || name.IsNull() || name.IsUnknown() {
		return
	}
	resp.PlanValue = name
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := ram.CreateCreateUserRequest()
	request.RegionId = r.client.RegionId
	request.UserName = plan.Name.ValueString()
	if isKnown(plan.DisplayName) {
		request.DisplayName = plan.DisplayName.ValueString()
	}
	if isKnown(plan.Mobile) {
		request.MobilePhone = plan.Mobile.ValueString()
	}
	if isKnown(plan.Email) {
		request.Email = plan.Email.ValueString()
	}
	if isKnown(plan.Comments) {
		request.Comments = plan.Comments.ValueString()
	}

	raw, err := r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.CreateUser(request)
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating alicloud_ram_user_v2", err.Error())
		return
	}
	createResp, _ := raw.(*ram.CreateUserResponse)

	user, found, err := r.describeUser(createResp.User.UserId)
	if err != nil {
		resp.Diagnostics.AddError("Error reading alicloud_ram_user_v2 after create", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("alicloud_ram_user_v2 not found after create", fmt.Sprintf("user id %s", createResp.User.UserId))
		return
	}

	applyUser(&plan, user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	setUserIdentity(ctx, resp.Identity, plan.ID, &resp.Diagnostics)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, found, err := r.describeUser(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading alicloud_ram_user_v2", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	applyUser(&state, user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	setUserIdentity(ctx, resp.Identity, state.ID, &resp.Diagnostics)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := map[string]interface{}{
		"UserName":    state.Name.ValueString(),
		"NewUserName": plan.Name.ValueString(),
	}
	if isKnown(plan.DisplayName) {
		request["NewDisplayName"] = plan.DisplayName.ValueString()
	}
	if isKnown(plan.Mobile) {
		request["NewMobilePhone"] = plan.Mobile.ValueString()
	}
	if isKnown(plan.Email) {
		request["NewEmail"] = plan.Email.ValueString()
	}
	if isKnown(plan.Comments) {
		request["NewComments"] = plan.Comments.ValueString()
	}

	if _, err := r.client.RpcPost("Ram", "2015-05-01", "UpdateUser", nil, request, false); err != nil {
		resp.Diagnostics.AddError("Error updating alicloud_ram_user_v2", err.Error())
		return
	}

	user, found, err := r.describeUser(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading alicloud_ram_user_v2 after update", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	applyUser(&plan, user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	setUserIdentity(ctx, resp.Identity, plan.ID, &resp.Diagnostics)
}

// setUserIdentity writes the resource identity (id) when the request carries an
// identity object (Terraform >= 1.12 with identity support).
func setUserIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, id types.String, diags *diag.Diagnostics) {
	if identity == nil {
		return
	}
	diags.Append(identity.Set(ctx, userIdentityModel{ID: id})...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, found, err := r.describeUser(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading alicloud_ram_user_v2 before delete", err.Error())
		return
	}
	if !found {
		return
	}
	userName := user.UserName

	if state.Force.ValueBool() {
		if err := r.forceCleanup(userName); err != nil {
			resp.Diagnostics.AddError("Error cleaning up alicloud_ram_user_v2 dependencies", err.Error())
			return
		}
	}

	deleteReq := ram.CreateDeleteUserRequest()
	deleteReq.RegionId = r.client.RegionId
	deleteReq.UserName = userName
	_, err = r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.DeleteUser(deleteReq)
	})
	if err != nil {
		if errorHasPrefix(err, "DeleteConflict") {
			resp.Diagnostics.AddError(
				"Error deleting alicloud_ram_user_v2",
				"The user cannot be deleted while it has access keys, a login profile, group memberships, policies or an MFA device attached. Set force = true to remove them automatically.",
			)
			return
		}
		resp.Diagnostics.AddError("Error deleting alicloud_ram_user_v2", err.Error())
		return
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}

// describeUser mirrors RamService.DescribeRamUser: it resolves a user by ID or
// name via ListUsers, then fetches details via GetUser. The bool return is false
// when the user does not exist.
func (r *userResource) describeUser(id string) (*ram.User, bool, error) {
	listReq := ram.CreateListUsersRequest()
	listReq.RegionId = r.client.RegionId
	listReq.MaxItems = requests.NewInteger(100)

	var userName string
	for {
		raw, err := r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
			return c.ListUsers(listReq)
		})
		if err != nil {
			return nil, false, err
		}
		listResp, _ := raw.(*ram.ListUsersResponse)
		for _, u := range listResp.Users.User {
			// The id has been the user id since v1.44.0; also match by name for
			// backward compatibility and imports.
			if u.UserId == id || u.UserName == id {
				userName = u.UserName
				break
			}
		}
		if userName != "" || !listResp.IsTruncated {
			break
		}
		listReq.Marker = listResp.Marker
	}

	if userName == "" {
		return nil, false, nil
	}

	getReq := ram.CreateGetUserRequest()
	getReq.RegionId = r.client.RegionId
	getReq.UserName = userName
	raw, err := r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.GetUser(getReq)
	})
	if err != nil {
		if errorHasPrefix(err, "EntityNotExist") {
			return nil, false, nil
		}
		return nil, false, err
	}
	getResp, _ := raw.(*ram.GetUserResponse)
	return &getResp.User, true, nil
}

// forceCleanup removes the dependencies that would otherwise block user
// deletion, mirroring the SDKv2 resource's force path. EntityNotExist errors are
// ignored so cleanup is idempotent.
func (r *userResource) forceCleanup(userName string) error {
	// Access keys.
	akReq := ram.CreateListAccessKeysRequest()
	akReq.RegionId = r.client.RegionId
	akReq.UserName = userName
	raw, err := r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.ListAccessKeys(akReq)
	})
	if err != nil {
		return err
	}
	if aks, _ := raw.(*ram.ListAccessKeysResponse); aks != nil {
		for _, k := range aks.AccessKeys.AccessKey {
			delReq := ram.CreateDeleteAccessKeyRequest()
			delReq.RegionId = r.client.RegionId
			delReq.UserName = userName
			delReq.UserAccessKeyId = k.AccessKeyId
			if _, err := r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
				return c.DeleteAccessKey(delReq)
			}); err != nil && !errorHasPrefix(err, "EntityNotExist") {
				return err
			}
		}
	}

	// Attached policies.
	polReq := ram.CreateListPoliciesForUserRequest()
	polReq.RegionId = r.client.RegionId
	polReq.UserName = userName
	raw, err = r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.ListPoliciesForUser(polReq)
	})
	if err != nil {
		return err
	}
	if pols, _ := raw.(*ram.ListPoliciesForUserResponse); pols != nil {
		for _, p := range pols.Policies.Policy {
			detachReq := ram.CreateDetachPolicyFromUserRequest()
			detachReq.RegionId = r.client.RegionId
			detachReq.UserName = userName
			detachReq.PolicyName = p.PolicyName
			detachReq.PolicyType = p.PolicyType
			if _, err := r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
				return c.DetachPolicyFromUser(detachReq)
			}); err != nil && !errorHasPrefix(err, "EntityNotExist") {
				return err
			}
		}
	}

	// Group memberships.
	grpReq := ram.CreateListGroupsForUserRequest()
	grpReq.RegionId = r.client.RegionId
	grpReq.UserName = userName
	raw, err = r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.ListGroupsForUser(grpReq)
	})
	if err != nil {
		return err
	}
	if grps, _ := raw.(*ram.ListGroupsForUserResponse); grps != nil {
		for _, g := range grps.Groups.Group {
			removeReq := ram.CreateRemoveUserFromGroupRequest()
			removeReq.RegionId = r.client.RegionId
			removeReq.UserName = userName
			removeReq.GroupName = g.GroupName
			if _, err := r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
				return c.RemoveUserFromGroup(removeReq)
			}); err != nil && !errorHasPrefix(err, "EntityNotExist") {
				return err
			}
		}
	}

	// Login profile.
	lpReq := ram.CreateDeleteLoginProfileRequest()
	lpReq.RegionId = r.client.RegionId
	lpReq.UserName = userName
	if _, err := r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.DeleteLoginProfile(lpReq)
	}); err != nil && !errorHasPrefix(err, "EntityNotExist") {
		return err
	}

	// MFA device.
	mfaReq := ram.CreateUnbindMFADeviceRequest()
	mfaReq.UserName = userName
	if _, err := r.client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.UnbindMFADevice(mfaReq)
	}); err != nil && !errorHasPrefix(err, "EntityNotExist") {
		return err
	}

	return nil
}

// applyUser writes the API-returned user fields into the model. Force is
// config-driven and left untouched.
func applyUser(m *userModel, u *ram.User) {
	m.ID = types.StringValue(u.UserId)
	m.Name = types.StringValue(u.UserName)
	m.DisplayName = types.StringValue(u.DisplayName)
	m.Mobile = types.StringValue(u.MobilePhone)
	m.Email = types.StringValue(u.Email)
	m.Comments = types.StringValue(u.Comments)
}

// isKnown reports whether a framework string value carries a usable value.
func isKnown(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown()
}

// errorHasPrefix reports whether err is an Alibaba Cloud SDK error whose code
// starts with prefix (e.g. "EntityNotExist", "DeleteConflict").
func errorHasPrefix(err error, prefix string) bool {
	var sdkErr sdkerrors.Error
	if errors.As(err, &sdkErr) {
		return strings.HasPrefix(sdkErr.ErrorCode(), prefix)
	}
	return false
}
