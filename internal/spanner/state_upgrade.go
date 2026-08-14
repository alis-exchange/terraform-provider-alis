package spanner

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// resourceSchemaVersion is stamped on every resource schema. Every release
// before the stamp — all of v1.x and the 2.0.0 betas — wrote state at the
// implicit version 0, so each resource registers a version-0 upgrader below;
// without one, Terraform refuses to read that state at all.
const resourceSchemaVersion int64 = 2

var (
	_ resource.ResourceWithUpgradeState = &spannerTableResource{}
	_ resource.ResourceWithUpgradeState = &spannerTableIndexResource{}
	_ resource.ResourceWithUpgradeState = &spannerTableForeignKeyResource{}
	_ resource.ResourceWithUpgradeState = &spannerTableTtlPolicyResource{}
	_ resource.ResourceWithUpgradeState = &tableIamBindingResource{}
	_ resource.ResourceWithUpgradeState = &databaseRoleResource{}
	_ resource.ResourceWithUpgradeState = &databaseSequenceResource{}
)

// passthroughUpgradeV0 upgrades version-0 state for resources whose attribute
// shape has not changed since v1.5.x: prior state decoded against the current
// schema IS the upgraded state. Attributes the old release did not know (the
// timeouts block) decode as null, and the framework already ignores JSON
// fields absent from the schema.
func passthroughUpgradeV0(ctx context.Context, r resource.Resource) map[int64]resource.StateUpgrader {
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	current := schemaResp.Schema

	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &current,
			StateUpgrader: func(_ context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				resp.State.Raw = req.State.Raw
			},
		},
	}
}

func (r *spannerTableIndexResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return passthroughUpgradeV0(ctx, r)
}

func (r *spannerTableForeignKeyResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return passthroughUpgradeV0(ctx, r)
}

func (r *spannerTableTtlPolicyResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return passthroughUpgradeV0(ctx, r)
}

func (r *tableIamBindingResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return passthroughUpgradeV0(ctx, r)
}

func (r *databaseRoleResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return passthroughUpgradeV0(ctx, r)
}

func (r *databaseSequenceResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return passthroughUpgradeV0(ctx, r)
}

// spannerTableV0Column is a column as any version-0 release may have written
// it: the current fields plus the v1.x-only attributes that v2 removed.
// Version-0 state comes in two shapes — v1.5.x (removed attributes present,
// is_stored absent) and 2.0.0-beta (the reverse) — so the prior schema is
// the superset of both and the absent side simply decodes as null.
type spannerTableV0Column struct {
	Name           types.String `tfsdk:"name"`
	IsPrimaryKey   types.Bool   `tfsdk:"is_primary_key"`
	IsComputed     types.Bool   `tfsdk:"is_computed"`
	ComputationDdl types.String `tfsdk:"computation_ddl"`
	IsStored       types.Bool   `tfsdk:"is_stored"`
	AutoUpdateTime types.Bool   `tfsdk:"auto_update_time"`
	Type           types.String `tfsdk:"type"`
	Size           types.Int64  `tfsdk:"size"`
	Required       types.Bool   `tfsdk:"required"`
	DefaultValue   types.String `tfsdk:"default_value"`
	ProtoPackage   types.String `tfsdk:"proto_package"`

	AutoIncrement  types.Bool   `tfsdk:"auto_increment"`
	Unique         types.Bool   `tfsdk:"unique"`
	Precision      types.Int64  `tfsdk:"precision"`
	Scale          types.Int64  `tfsdk:"scale"`
	FileDescriptor types.String `tfsdk:"file_descriptor"`
}

type spannerTableV0Model struct {
	Name           types.String            `tfsdk:"name"`
	Project        types.String            `tfsdk:"project"`
	Instance       types.String            `tfsdk:"instance"`
	Database       types.String            `tfsdk:"database"`
	Schema         *spannerTableV0Schema   `tfsdk:"schema"`
	Interleave     *spannerTableInterleave `tfsdk:"interleave"`
	PreventDestroy types.Bool              `tfsdk:"prevent_destroy"`
	Timeouts       timeouts.Value          `tfsdk:"timeouts"`
}

type spannerTableV0Schema struct {
	Columns types.List `tfsdk:"columns"`
}

// spannerTableV0PriorSchema describes version-0 table state for decoding
// only — types must match what was stored, but validators, plan modifiers
// and requiredness never run against prior state, so they are omitted.
func spannerTableV0PriorSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Optional: true},
			"project":  schema.StringAttribute{Optional: true},
			"instance": schema.StringAttribute{Optional: true},
			"database": schema.StringAttribute{Optional: true},
			"schema": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"columns": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name":             schema.StringAttribute{Optional: true},
								"is_primary_key":   schema.BoolAttribute{Optional: true},
								"is_computed":      schema.BoolAttribute{Optional: true},
								"computation_ddl":  schema.StringAttribute{Optional: true},
								"is_stored":        schema.BoolAttribute{Optional: true},
								"auto_update_time": schema.BoolAttribute{Optional: true},
								"type":             schema.StringAttribute{Optional: true},
								"size":             schema.Int64Attribute{Optional: true},
								"required":         schema.BoolAttribute{Optional: true},
								"default_value":    schema.StringAttribute{Optional: true},
								"proto_package":    schema.StringAttribute{Optional: true},
								"auto_increment":   schema.BoolAttribute{Optional: true},
								"unique":           schema.BoolAttribute{Optional: true},
								"precision":        schema.Int64Attribute{Optional: true},
								"scale":            schema.Int64Attribute{Optional: true},
								"file_descriptor":  schema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			"interleave": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"parent_table": schema.StringAttribute{Optional: true},
					"on_delete":    schema.StringAttribute{Optional: true},
				},
			},
			"prevent_destroy": schema.BoolAttribute{Optional: true},
		},
	}
}

func (r *spannerTableResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	priorSchema := spannerTableV0PriorSchema(ctx)

	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &priorSchema,
			StateUpgrader: upgradeSpannerTableStateV0,
		},
	}
}

func upgradeSpannerTableStateV0(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	var prior spannerTableV0Model
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	upgraded := spannerTableModel{
		Name:           prior.Name,
		Project:        prior.Project,
		Instance:       prior.Instance,
		Database:       prior.Database,
		Interleave:     prior.Interleave,
		PreventDestroy: prior.PreventDestroy,
		Timeouts:       prior.Timeouts,
	}

	if prior.Schema != nil {
		var priorColumns []spannerTableV0Column
		resp.Diagnostics.Append(prior.Schema.Columns.ElementsAs(ctx, &priorColumns, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		columns := make([]spannerTableColumn, 0, len(priorColumns))
		for _, c := range priorColumns {
			// v1.x auto_increment created a live `<table>_seq` sequence and a
			// DEFAULT on the column. Nothing in the upgraded state or config
			// expresses that anymore, so the very next apply would silently
			// drop the default from the database — warn before that happens.
			if c.AutoIncrement.ValueBool() {
				resp.Diagnostics.AddWarning(
					fmt.Sprintf("Column %q used the removed auto_increment attribute", c.Name.ValueString()),
					fmt.Sprintf("Provider v1.x implemented auto_increment as a sequence named %[1]q with a column default. "+
						"To keep values auto-generating, manage the sequence with an alis_google_spanner_database_sequence resource and set "+
						"default_value = \"GET_NEXT_SEQUENCE_VALUE(SEQUENCE %[1]s)\" on column %[2]q. "+
						"Applying without default_value removes the default from the database.",
						prior.Name.ValueString()+"_seq", c.Name.ValueString()),
				)
			}
			if c.Unique.ValueBool() {
				resp.Diagnostics.AddWarning(
					fmt.Sprintf("Column %q used the removed unique attribute", c.Name.ValueString()),
					"Manage a unique index explicitly with an alis_google_spanner_table_index resource with unique = true.",
				)
			}
			// v1.x quoted string defaults on the user's behalf (DEFAULT ('hello'));
			// v2 emits default_value verbatim inside DEFAULT (...), so an unquoted
			// literal fails DDL parsing on the next apply.
			if defaultValue := c.DefaultValue.ValueString(); defaultValue != "" &&
				(c.Type.ValueString() == "STRING" || c.Type.ValueString() == "BYTES") &&
				!strings.HasPrefix(defaultValue, "'") {
				resp.Diagnostics.AddWarning(
					fmt.Sprintf("Column %q has a default_value that now needs quoting", c.Name.ValueString()),
					fmt.Sprintf("default_value is now the raw expression inside Spanner's DEFAULT (...). "+
						"Provider v1.x quoted string literals for you; wrap the value in single quotes, e.g. default_value = \"'%s'\".",
						defaultValue),
				)
			}
			columns = append(columns, spannerTableColumn{
				Name:           c.Name,
				IsPrimaryKey:   c.IsPrimaryKey,
				IsComputed:     c.IsComputed,
				ComputationDdl: c.ComputationDdl,
				IsStored:       c.IsStored,
				AutoUpdateTime: c.AutoUpdateTime,
				Type:           c.Type,
				Size:           c.Size,
				Required:       c.Required,
				DefaultValue:   c.DefaultValue,
				ProtoPackage:   c.ProtoPackage,
			})
		}

		columnsList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: spannerTableColumn{}.attrTypes()}, columns)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		upgraded.Schema = &spannerTableSchema{Columns: columnsList}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &upgraded)...)
}
