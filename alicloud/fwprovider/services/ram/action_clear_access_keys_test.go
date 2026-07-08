package ram_test

import (
	"fmt"
	"testing"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/fwprovider/acctest"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAliCloudRamUserClearAccessKeysAction exercises the framework Action via
// HCL: a resource lifecycle action_trigger invokes
// alicloud_ram_user_clear_access_keys on update. The test creates an access key
// out of band (actions/resources here don't manage access keys), triggers the
// action with an update, and asserts the keys were cleared. Requires
// Terraform >= 1.14.
func TestAccAliCloudRamUserClearAccessKeysAction(t *testing.T) {
	acctest.PreCheck(t)
	client := acctest.SharedClient(t)
	name := fmt.Sprintf("tf-testacc-ramkeys-%d", sdkacctest.RandInt())
	resourceName := "alicloud_ram_user_v2.default"

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
				// Create the user; the after_update action does not fire yet.
				Config: testUserActionConfig(name, "Display One"),
				Check:  resource.TestCheckResourceAttr(resourceName, "name", name),
			},
			{
				// Create an access key out of band, then update the user, which
				// fires the after_update action_trigger to clear the keys.
				PreConfig: func() {
					if err := createAccessKey(client, name); err != nil {
						t.Fatalf("creating access key: %s", err)
					}
					if n, err := accessKeyCount(client, name); err != nil || n == 0 {
						t.Fatalf("expected an access key before update (n=%d err=%v)", n, err)
					}
				},
				Config: testUserActionConfig(name, "Display Two"),
				Check: func(s *terraform.State) error {
					n, err := accessKeyCount(client, name)
					if err != nil {
						return err
					}
					if n != 0 {
						return fmt.Errorf("expected 0 access keys after the action, got %d", n)
					}
					return nil
				},
			},
		},
	})
}

func testUserActionConfig(name, displayName string) string {
	return fmt.Sprintf(`
action "alicloud_ram_user_clear_access_keys" "clear" {
  config {
    user_name = alicloud_ram_user_v2.default.name
  }
}

resource "alicloud_ram_user_v2" "default" {
  name         = "%s"
  display_name = "%s"
  force        = true

  lifecycle {
    action_trigger {
      events  = [after_update]
      actions = [action.alicloud_ram_user_clear_access_keys.clear]
    }
  }
}
`, name, displayName)
}

func createAccessKey(client *connectivity.AliyunClient, userName string) error {
	request := ram.CreateCreateAccessKeyRequest()
	request.RegionId = client.RegionId
	request.UserName = userName
	_, err := client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.CreateAccessKey(request)
	})
	return err
}

func accessKeyCount(client *connectivity.AliyunClient, userName string) (int, error) {
	request := ram.CreateListAccessKeysRequest()
	request.RegionId = client.RegionId
	request.UserName = userName
	raw, err := client.WithRamClient(func(c *ram.Client) (interface{}, error) {
		return c.ListAccessKeys(request)
	})
	if err != nil {
		return 0, err
	}
	resp, _ := raw.(*ram.ListAccessKeysResponse)
	return len(resp.AccessKeys.AccessKey), nil
}
