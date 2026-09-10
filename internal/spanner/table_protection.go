package spanner

import (
	"context"

	"terraform-provider-alis/internal/spanner/names"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// tableReplaceDescription documents the replace-only attributes. The phrase
// "destroy and recreate" is what identifies a replace modifier to the schema
// tests; tfplugindocs renders attribute descriptions, never these.
const tableReplaceDescription = "If this value changes, Terraform will destroy and recreate the table. " +
	"The plan fails instead while the prior state has prevent_destroy set to true."

// tableProtection reads prevent_destroy and the fully qualified table name
// from prior state. A null state (the table does not exist yet, or a plan
// modifier invoked without one) is unprotected, as is a null prevent_destroy:
// state seeded by ImportState carries no value until the first apply. The
// state is checked for null before its schema is touched, because a zero
// tfsdk.State has no schema to read paths from.
func tableProtection(ctx context.Context, state tfsdk.State) (string, bool, diag.Diagnostics) {
	if state.Raw.IsNull() {
		return "", false, nil
	}

	var model spannerTableModel
	diags := state.Get(ctx, &model)
	if diags.HasError() {
		return "", false, diags
	}

	tableName := names.TableName{
		Project:  model.Project.ValueString(),
		Instance: model.Instance.ValueString(),
		Database: model.Database.ValueString(),
		Table:    model.Name.ValueString(),
	}.String()

	return tableName, model.PreventDestroy.ValueBool(), diags
}

// tableProtectionDetail opens both protection diagnostics. Operators and the
// acceptance suite match on "protected from deletion".
func tableProtectionDetail(tableName string) string {
	return "Table (" + tableName + ") is protected from deletion by the Terraform configuration: " +
		"`prevent_destroy` is true in the prior state."
}

// guardTableDestroy reports an error while the prior state protects the table
// from deletion. ModifyPlan applies it to destroy plans so the failure lands
// before anything is destroyed; Delete applies it again at apply time, for
// destroys that reach the provider without a destroy plan.
func guardTableDestroy(ctx context.Context, state tfsdk.State) diag.Diagnostics {
	tableName, protected, diags := tableProtection(ctx, state)
	if diags.HasError() || !protected {
		return diags
	}

	diags.Append(diag.NewAttributeErrorDiagnostic(
		path.Root("prevent_destroy"),
		"Table Protected From Deletion",
		tableProtectionDetail(tableName)+" To destroy it, set `prevent_destroy` to false and apply that change on its own, "+
			"in a plan that makes no other change to the table, then plan the destroy again.",
	))

	return diags
}

// guardTableReplace reports an error when a change at attrPath would replace a
// table the prior state protects. Only prior state is consulted: setting
// `prevent_destroy` to false in the same change as the replacement does not
// lift the protection, while a change to `prevent_destroy` alone never
// replaces the table and so never reaches this guard.
func guardTableReplace(ctx context.Context, state tfsdk.State, attrPath path.Path) diag.Diagnostics {
	tableName, protected, diags := tableProtection(ctx, state)
	if diags.HasError() || !protected {
		return diags
	}

	diags.Append(diag.NewAttributeErrorDiagnostic(
		attrPath,
		"Table Protected From Replacement",
		tableProtectionDetail(tableName)+" The change to `"+attrPath.String()+"` would destroy and recreate the table. "+
			"To replace it, set `prevent_destroy` to false and apply that change on its own, "+
			"in a plan that does not replace the table, then apply the replacement.",
	))

	return diags
}

// tableStringRequiresReplace is the RequiresReplaceIf handler for the
// replace-only string attributes: any change replaces the table, and the
// replace decision is where destroy protection has to be enforced, because
// resource-level ModifyPlan is handed no attribute replace paths.
func tableStringRequiresReplace(
	ctx context.Context,
	req planmodifier.StringRequest,
	resp *stringplanmodifier.RequiresReplaceIfFuncResponse,
) {
	resp.RequiresReplace = true
	resp.Diagnostics.Append(guardTableReplace(ctx, req.State, req.Path)...)
}

// tableObjectRequiresReplace is the RequiresReplaceIf handler for the
// interleave object. The nested parent_table and on_delete attributes keep the
// unconditional RequiresReplace: a nested change also changes the object, so
// guarding both levels would refuse one plan twice under two paths.
func tableObjectRequiresReplace(
	ctx context.Context,
	req planmodifier.ObjectRequest,
	resp *objectplanmodifier.RequiresReplaceIfFuncResponse,
) {
	resp.RequiresReplace = true
	resp.Diagnostics.Append(guardTableReplace(ctx, req.State, req.Path)...)
}
