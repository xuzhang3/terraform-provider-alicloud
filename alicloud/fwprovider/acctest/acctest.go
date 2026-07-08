// Package acctest provides shared helpers for terraform-plugin-framework
// acceptance tests living in the per-product fwprovider subpackages: a
// credential precheck and a client builder. It intentionally does NOT import
// fwprovider/alicloud, so tests that use terraform-plugin-testing (e.g. the
// ephemeral resource's statecheck test) don't link terraform-plugin-sdk/v2's
// helper/resource, whose init would collide with terraform-plugin-testing's
// identical sweep-flag registration.
package acctest

import (
	"os"
	"sync"
	"testing"

	"github.com/aliyun/credentials-go/credentials"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/fwprovider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
)

// FrameworkProtoV6ProviderFactories returns protocol v6 factories for
// framework-only acceptance tests: the standalone framework provider (serving
// the framework resources with the supplied client) under "alicloud", plus the
// terraform-plugin-testing "echo" provider for asserting ephemeral values.
//
// It serves the framework provider standalone (not muxed) on purpose: a
// statecheck test cannot link the SDKv2 provider, whose helper/resource init
// collides with terraform-plugin-testing's sweep flags.
func FrameworkProtoV6ProviderFactories(client *connectivity.AliyunClient) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"alicloud": providerserver.NewProtocol6WithError(fwprovider.NewStandaloneProvider(client)),
		"echo":     echoprovider.NewProviderServer(),
	}
}

// PreCheck skips the test unless the credentials needed to run acceptance tests
// are present in the environment.
func PreCheck(t *testing.T) {
	for _, key := range []string{"ALICLOUD_ACCESS_KEY", "ALICLOUD_SECRET_KEY", "ALICLOUD_REGION"} {
		if os.Getenv(key) == "" {
			t.Skipf("%s must be set for acceptance tests", key)
		}
	}
}

// SharedClient builds an AliyunClient from environment credentials for
// out-of-band checks (e.g. CheckDestroy), mirroring the sweeper's
// sharedClientForRegion.
func SharedClient(t *testing.T) *connectivity.AliyunClient {
	region := os.Getenv("ALICLOUD_REGION")
	accessKey := os.Getenv("ALICLOUD_ACCESS_KEY")
	secretKey := os.Getenv("ALICLOUD_SECRET_KEY")
	securityToken := os.Getenv("ALICLOUD_SECURITY_TOKEN")

	var endpoints, signVersion sync.Map
	conf := connectivity.Config{
		Region:      connectivity.Region(region),
		RegionId:    region,
		AccessKey:   accessKey,
		SecretKey:   secretKey,
		Protocol:    "HTTPS",
		Endpoints:   &endpoints,
		SignVersion: &signVersion,
	}
	if securityToken != "" {
		conf.SecurityToken = securityToken
	}

	credentialConfig := new(credentials.Config).SetType("access_key").SetAccessKeyId(accessKey).SetAccessKeySecret(secretKey)
	if securityToken != "" {
		credentialConfig.SetType("sts").SetSecurityToken(securityToken)
	}
	credential, err := credentials.NewCredential(credentialConfig)
	if err != nil {
		t.Fatalf("building credential for acceptance test: %s", err)
	}
	conf.Credential = credential

	client, err := conf.Client()
	if err != nil {
		t.Fatalf("building client for acceptance test: %s", err)
	}
	return client
}
