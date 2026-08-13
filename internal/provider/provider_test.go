package provider_test

import (
	"context"
	"testing"

	"terraform-provider-alis/internal/provider"

	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestProvider_Metadata(t *testing.T) {
	p := provider.NewProvider("1.2.3")()

	resp := fwprovider.MetadataResponse{}
	p.Metadata(context.Background(), fwprovider.MetadataRequest{}, &resp)

	if resp.TypeName != "alis" {
		t.Errorf("provider type name = %q, want %q", resp.TypeName, "alis")
	}
	if resp.Version != "1.2.3" {
		t.Errorf("provider version = %q, want %q", resp.Version, "1.2.3")
	}
}

func TestProvider_SchemaValid(t *testing.T) {
	ctx := context.Background()
	p := provider.NewProvider("test")()

	resp := fwprovider.SchemaResponse{}
	p.Schema(ctx, fwprovider.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("provider schema diagnostics: %v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Errorf("provider schema implementation: %v", diags)
	}
}

func TestProvider_ResourceSchemasValid(t *testing.T) {
	ctx := context.Background()
	p := provider.NewProvider("test")()

	for _, newResource := range p.Resources(ctx) {
		r := newResource()

		metaResp := fwresource.MetadataResponse{}
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "alis"}, &metaResp)

		schemaResp := fwresource.SchemaResponse{}
		r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)

		if schemaResp.Diagnostics.HasError() {
			t.Errorf("%s: schema diagnostics: %v", metaResp.TypeName, schemaResp.Diagnostics)
			continue
		}
		if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Errorf("%s: schema implementation: %v", metaResp.TypeName, diags)
		}
	}
}

func TestProvider_DataSourceSchemasValid(t *testing.T) {
	ctx := context.Background()
	p := provider.NewProvider("test")()

	for _, newDataSource := range p.DataSources(ctx) {
		d := newDataSource()

		metaResp := fwdatasource.MetadataResponse{}
		d.Metadata(ctx, fwdatasource.MetadataRequest{ProviderTypeName: "alis"}, &metaResp)

		schemaResp := fwdatasource.SchemaResponse{}
		d.Schema(ctx, fwdatasource.SchemaRequest{}, &schemaResp)

		if schemaResp.Diagnostics.HasError() {
			t.Errorf("%s: schema diagnostics: %v", metaResp.TypeName, schemaResp.Diagnostics)
			continue
		}
		if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Errorf("%s: schema implementation: %v", metaResp.TypeName, diags)
		}
	}
}
