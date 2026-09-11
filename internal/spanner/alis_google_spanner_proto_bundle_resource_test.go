package spanner

import (
	"context"
	"slices"
	"strings"
	"testing"

	"terraform-provider-alis/internal/spanner/conn/connfake"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/spanner/services"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const protoBundleFixture = "schema/testdata/proto_bundle/owned_including_imports.fds"

// protoBundleValue decodes JSON into a value of the proto bundle schema, the
// way the framework decodes config and plan arriving over the wire.
// Attributes absent from the JSON decode as null.
func protoBundleValue(t *testing.T, jsonDoc string) tftypes.Value {
	t.Helper()
	sch := resourceSchema(t, NewProtoBundleResource())
	raw, err := (&tfprotov6.RawState{JSON: []byte(jsonDoc)}).UnmarshalWithOpts(
		sch.Type().TerraformType(context.Background()),
		tfprotov6.UnmarshalOpts{ValueFromJSONOpts: tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true}},
	)
	if err != nil {
		t.Fatalf("decoding proto bundle value: %v", err)
	}
	return raw
}

func protoBundleResourceWithFake(t *testing.T) *protoBundleResource {
	t.Helper()
	return &protoBundleResource{service: services.NewSpannerService(connfake.New())}
}

func TestProtoBundleSchema_IdentityRequiresReplace(t *testing.T) {
	resp := &resource.SchemaResponse{}
	NewProtoBundleResource().Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	requiresReplace := func(name string) bool {
		for _, desc := range planModifierDescriptions(t, resp.Schema.Attributes[name]) {
			if strings.Contains(desc, "destroy and recreate") {
				return true
			}
		}
		return false
	}
	for _, name := range []string{"project", "instance", "database"} {
		if !requiresReplace(name) {
			t.Errorf("%s must require replace: the bundle lives in one database", name)
		}
	}
	for _, name := range []string{"packages", "sources"} {
		if requiresReplace(name) {
			t.Errorf("%s must update in place; a replace would delete owned types under live columns", name)
		}
	}
	for _, name := range []string{"descriptors_sha256", "types"} {
		attr, ok := resp.Schema.Attributes[name]
		if !ok || !attr.IsComputed() || attr.IsOptional() || attr.IsRequired() {
			t.Errorf("%s must be computed-only", name)
		}
	}
}

func TestProtoBundleValidateConfig_SourceExactlyOne(t *testing.T) {
	ctx := context.Background()
	sch := resourceSchema(t, NewProtoBundleResource())
	r := NewProtoBundleResource().(resource.ResourceWithValidateConfig)
	validate := func(jsonDoc string) string {
		resp := &resource.ValidateConfigResponse{}
		r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Raw: protoBundleValue(t, jsonDoc), Schema: sch}}, resp)
		return diagText(resp.Diagnostics)
	}
	base := `"project":"p","instance":"i","database":"d","packages":["tftest.v1"],`

	if got := validate(
		`{` + base + `"sources":[{"local_path":"x","gcs_uri":"gs://b/o"}]}`,
	); !strings.Contains(got, "local_path") ||
		!strings.Contains(got, "gcs_uri") {
		t.Errorf("both set: diagnostics = %q, want one naming local_path and gcs_uri", got)
	}
	if got := validate(`{` + base + `"sources":[{}]}`); !strings.Contains(got, "local_path") || !strings.Contains(got, "gcs_uri") {
		t.Errorf("neither set: diagnostics = %q, want one naming local_path and gcs_uri", got)
	}
	if got := validate(`{` + base + `"sources":[{"local_path":"x"},{"gcs_uri":"gs://b/o"}]}`); got != "" {
		t.Errorf("valid config: unexpected diagnostics %q", got)
	}
}

func TestProtoBundleModifyPlan(t *testing.T) {
	ctx := context.Background()
	sch := resourceSchema(t, NewProtoBundleResource())
	r := protoBundleResourceWithFake(t)
	planJSON := `{"project":"p","instance":"i","database":"d","packages":["tftest.v1"],"sources":[{"local_path":"` + protoBundleFixture + `"}]}`

	t.Run("sets hash and owned types from the sources", func(t *testing.T) {
		planned := protoBundleValue(t, planJSON)
		req := resource.ModifyPlanRequest{
			Config: tfsdk.Config{Raw: planned, Schema: sch},
			Plan:   tfsdk.Plan{Raw: planned, Schema: sch},
			State:  tfsdk.State{Raw: tftypes.NewValue(sch.Type().TerraformType(ctx), nil), Schema: sch},
		}
		resp := &resource.ModifyPlanResponse{Plan: req.Plan}
		r.ModifyPlan(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("ModifyPlan: %s", diagText(resp.Diagnostics))
		}
		var sha string
		resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("descriptors_sha256"), &sha)...)
		var typeNames []string
		resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("types"), &typeNames)...)
		if resp.Diagnostics.HasError() {
			t.Fatalf("reading planned attributes: %s", diagText(resp.Diagnostics))
		}
		set, err := r.service.LoadDescriptorSources(ctx, protoBundleSourcesFromPaths(protoBundleFixture))
		if err != nil {
			t.Fatal(err)
		}
		if sha != set.Sha256 {
			t.Errorf("descriptors_sha256 = %q, want %q", sha, set.Sha256)
		}
		slices.Sort(typeNames)
		if want := []string{"tftest.v1.Kind", "tftest.v1.Simple", "tftest.v1.Simple.Nested"}; !slices.Equal(typeNames, want) {
			t.Errorf("types = %v, want owned %v", typeNames, want)
		}
	})

	t.Run("destroy plan is a no-op", func(t *testing.T) {
		null := tftypes.NewValue(sch.Type().TerraformType(ctx), nil)
		req := resource.ModifyPlanRequest{
			Config: tfsdk.Config{Raw: null, Schema: sch},
			Plan:   tfsdk.Plan{Raw: null, Schema: sch},
			State:  tfsdk.State{Raw: protoBundleValue(t, planJSON), Schema: sch},
		}
		resp := &resource.ModifyPlanResponse{Plan: req.Plan}
		r.ModifyPlan(ctx, req, resp)
		if resp.Diagnostics.HasError() || !resp.Plan.Raw.IsNull() {
			t.Fatalf("destroy: diagnostics %s, plan null = %v", diagText(resp.Diagnostics), resp.Plan.Raw.IsNull())
		}
	})

	t.Run("unreadable source fails the plan naming the path", func(t *testing.T) {
		planned := protoBundleValue(
			t,
			`{"project":"p","instance":"i","database":"d","packages":["tftest.v1"],"sources":[{"local_path":"/nonexistent/x.fds"}]}`,
		)
		req := resource.ModifyPlanRequest{
			Config: tfsdk.Config{Raw: planned, Schema: sch},
			Plan:   tfsdk.Plan{Raw: planned, Schema: sch},
			State:  tfsdk.State{Raw: tftypes.NewValue(sch.Type().TerraformType(ctx), nil), Schema: sch},
		}
		resp := &resource.ModifyPlanResponse{Plan: req.Plan}
		r.ModifyPlan(ctx, req, resp)
		if got := diagText(resp.Diagnostics); !resp.Diagnostics.HasError() || !strings.Contains(got, "/nonexistent/x.fds") {
			t.Fatalf("diagnostics = %q, want an error naming the path", got)
		}
	})
}

func protoBundleSourcesFromPaths(paths ...string) []schema.DescriptorSource {
	out := make([]schema.DescriptorSource, len(paths))
	for i, p := range paths {
		out[i] = schema.DescriptorSource{LocalPath: p}
	}
	return out
}
