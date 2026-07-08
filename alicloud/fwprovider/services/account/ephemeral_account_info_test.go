package account_test

import (
	"testing"

	"github.com/aliyun/terraform-provider-alicloud/alicloud/fwprovider/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAliCloudAccountInfoEphemeralResource exercises the ephemeral resource
// end to end with statecheck. Ephemeral values are never stored in state, so the
// result is piped through the terraform-plugin-testing "echo" provider (which
// writes it into echo.test.data) and asserted with statecheck.ExpectKnownValue.
//
// The provider factories come from acctest.FrameworkProtoV6ProviderFactories,
// which serves the framework provider standalone (not muxed) so the test does
// not link the SDKv2 provider. Requires Terraform >= 1.10 and credentials.
func TestAccAliCloudAccountInfoEphemeralResource(t *testing.T) {
	acctest.PreCheck(t)
	client := acctest.SharedClient(t)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		ProtoV6ProviderFactories: acctest.FrameworkProtoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccountInfoEphemeralConfig,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("account_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("region_id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

const testAccountInfoEphemeralConfig = `
ephemeral "alicloud_account_info" "test" {}

provider "echo" {
  data = ephemeral.alicloud_account_info.test
}

resource "echo" "test" {}
`
