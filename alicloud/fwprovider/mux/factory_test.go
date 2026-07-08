package mux

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

// TestMuxProviderSchemaParity verifies that the SDKv2 provider and the framework
// provider advertise an identical provider schema. tf5muxserver enforces this in
// GetProviderSchema and returns error diagnostics (including a cmp.Diff of the
// difference) when the schemas do not match. This test needs no credentials and
// is the CI gate against provider-schema drift.
func TestMuxProviderSchemaParity(t *testing.T) {
	ctx := context.Background()

	factory, _, err := ProtoV5ProviderServerFactory(ctx)
	if err != nil {
		t.Fatalf("creating mux provider server factory: %s", err)
	}

	resp, err := factory().GetProviderSchema(ctx, &tfprotov5.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("calling GetProviderSchema: %s", err)
	}

	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov5.DiagnosticSeverityError {
			t.Errorf("provider schema parity error: %s\n%s", d.Summary, d.Detail)
		}
	}
}
