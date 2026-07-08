package ram_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	sdkerrors "github.com/aliyun/alibaba-cloud-sdk-go/sdk/errors"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/fwprovider/acctest"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// These tests exercise the framework resource through a standalone framework
// provider (protocol v6) served by acctest.FrameworkProtoV6ProviderFactories,
// using terraform-plugin-testing. They deliberately do not go through the mux:
// the mux links the SDKv2 provider, whose helper/resource init collides with
// terraform-plugin-testing's sweep flags, and the List/query test requires
// terraform-plugin-testing. The mux itself is covered by the parity test in the
// fwprovider/mux package.

func TestAccAliCloudRamUserV2_basic(t *testing.T) {
	acctest.PreCheck(t)
	client := acctest.SharedClient(t)
	name := fmt.Sprintf("tf-testacc-ramuserv2-%d", sdkacctest.RandInt())
	nameUpdated := name + "-upd"
	resourceName := "alicloud_ram_user_v2.default"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.FrameworkProtoV6ProviderFactories(client),
		CheckDestroy: func(s *terraform.State) error {
			return testCheckUserDestroy(client, s)
		},
		Steps: []resource.TestStep{
			{
				Config: testUserConfig(name, "Display One", "comment one"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "display_name", "Display One"),
					resource.TestCheckResourceAttr(resourceName, "comments", "comment one"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force"},
			},
			{
				Config: testUserConfig(nameUpdated, "Display Two", "comment two"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", nameUpdated),
					resource.TestCheckResourceAttr(resourceName, "display_name", "Display Two"),
					resource.TestCheckResourceAttr(resourceName, "comments", "comment two"),
				),
			},
		},
	})
}

// TestAccAliCloudRamUserV2_displayNameDefault verifies the plan modifier that
// defaults display_name to the user name when display_name is not configured.
func TestAccAliCloudRamUserV2_displayNameDefault(t *testing.T) {
	acctest.PreCheck(t)
	client := acctest.SharedClient(t)
	name := fmt.Sprintf("tf-testacc-ramuserv2-%d", sdkacctest.RandInt())
	resourceName := "alicloud_ram_user_v2.default"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.FrameworkProtoV6ProviderFactories(client),
		CheckDestroy: func(s *terraform.State) error {
			return testCheckUserDestroy(client, s)
		},
		Steps: []resource.TestStep{
			{
				Config: testUserConfigNameOnly(name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", name),
					// display_name was not set, so the plan modifier defaults it to name.
					resource.TestCheckResourceAttrPair(resourceName, "display_name", resourceName, "name"),
				),
			},
		},
	})
}

// TestAccAliCloudRamUserV2_list exercises the framework List resource: it
// provisions a user, then runs `terraform query` on the alicloud_ram_user_v2
// list resource filtered by the user's name and asserts it is found. Requires
// Terraform >= 1.14.
func TestAccAliCloudRamUserV2_list(t *testing.T) {
	acctest.PreCheck(t)
	client := acctest.SharedClient(t)
	name := fmt.Sprintf("tf-testacc-ramuserv2-%d", sdkacctest.RandInt())

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: acctest.FrameworkProtoV6ProviderFactories(client),
		CheckDestroy: func(s *terraform.State) error {
			return testCheckUserDestroy(client, s)
		},
		Steps: []resource.TestStep{
			{
				// Provision the user that the query should discover.
				Config: testUserConfigNameOnly(name),
			},
			{
				Query:  true,
				Config: testUserListConfig(name),
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLengthAtLeast("alicloud_ram_user_v2.test", 1),
				},
			},
		},
	})
}

func testUserConfig(name, displayName, comments string) string {
	return fmt.Sprintf(`
resource "alicloud_ram_user_v2" "default" {
  name         = "%s"
  display_name = "%s"
  comments     = "%s"
  force        = true
}
`, name, displayName, comments)
}

func testUserConfigNameOnly(name string) string {
	return fmt.Sprintf(`
resource "alicloud_ram_user_v2" "default" {
  name  = "%s"
  force = true
}
`, name)
}

func testUserListConfig(nameRegex string) string {
	return fmt.Sprintf(`
provider "alicloud" {}

list "alicloud_ram_user_v2" "test" {
  provider = alicloud

  config {
    name_regex = "%s"
  }
}
`, nameRegex)
}

func testCheckUserDestroy(client *connectivity.AliyunClient, s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "alicloud_ram_user_v2" {
			continue
		}
		exists, err := userExists(client, rs.Primary.Attributes["name"])
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("alicloud_ram_user_v2 %q still exists", rs.Primary.Attributes["name"])
		}
	}
	return nil
}

func userExists(client *connectivity.AliyunClient, name string) (bool, error) {
	request := ram.CreateGetUserRequest()
	request.RegionId = client.RegionId
	request.UserName = name
	_, err := client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.GetUser(request)
	})
	if err != nil {
		var sdkErr sdkerrors.Error
		if errors.As(err, &sdkErr) && strings.HasPrefix(sdkErr.ErrorCode(), "EntityNotExist") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
