package custom_types

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Ensure the implementation satisfies the expected interfaces
var _ basetypes.StringTypable = JsonStringType{}
var _ basetypes.StringValuable = JsonStringValue{}
var _ basetypes.StringValuableWithSemanticEquals = JsonStringValue{}

type JsonStringType struct {
	basetypes.StringType
	// ... potentially other fields ...
}

type JsonStringValue struct {
	basetypes.StringValue
	// ... potentially other fields ...
}

// NewJsonStringValue creates a new JsonStringValue
func NewJsonStringValue(value string) JsonStringValue {
	return JsonStringValue{
		StringValue: types.StringValue(value),
	}
}

func (v JsonStringType) Equal(o attr.Type) bool {
	other, ok := o.(JsonStringType)

	if !ok {
		return false
	}

	return v.StringType.Equal(other.StringType)
}

func (v JsonStringValue) Equal(o attr.Value) bool {
	other, ok := o.(JsonStringValue)

	if !ok {
		return false
	}

	return v.StringValue.Equal(other.StringValue)
}

func (t JsonStringType) String() string {
	return "JsonStringType"
}

func (t JsonStringType) ValueFromString(ctx context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	value := JsonStringValue{
		StringValue: in,
	}

	return value, nil
}

func (t JsonStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)

	if err != nil {
		return nil, err
	}

	stringValue, ok := attrValue.(basetypes.StringValue)

	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}

	stringValuable, diags := t.ValueFromString(ctx, stringValue)

	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}

	return stringValuable, nil
}

func (t JsonStringType) ValueType(ctx context.Context) attr.Value {
	return JsonStringValue{}
}

func (t JsonStringType) validate(value string) error {
	var j interface{}
	err := json.Unmarshal([]byte(value), &j)
	return err
}

func (t JsonStringType) Validate(ctx context.Context, value tftypes.Value, valuePath path.Path) diag.Diagnostics {
	if value.IsNull() || !value.IsKnown() {
		return nil
	}

	var diags diag.Diagnostics
	var valueString string

	if err := value.As(&valueString); err != nil {
		diags.AddAttributeError(
			valuePath,
			"Invalid Terraform Value",
			"An unexpected error occurred while attempting to convert a Terraform value to a string. "+
				"This generally is an issue with the provider schema implementation. "+
				"Please contact the provider developers.\n\n"+
				"Path: "+valuePath.String()+"\n"+
				"Error: "+err.Error(),
		)

		return diags
	}

	if err := t.validate(valueString); err != nil {
		diags.AddAttributeError(
			valuePath,
			"Invalid JSON String Value",
			"An unexpected error occurred while converting a string value that was expected to be JSON string format. "+
				"The string should be a valid JSON string\n\n"+
				"Path: "+valuePath.String()+"\n"+
				"Given Value: "+valueString+"\n"+
				"Error: "+err.Error(),
		)

		return diags
	}

	return diags
}
func (v JsonStringValue) Type(ctx context.Context) attr.Type {
	return JsonStringType{}
}

func (v JsonStringValue) StringSemanticEquals(ctx context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The framework should always pass the correct value type, but always check
	newValue, ok := newValuable.(JsonStringValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+fmt.Sprintf("%T", v)+"\n"+
				"Got Value Type: "+fmt.Sprintf("%T", newValuable),
		)

		return false, diags
	}

	var priorJson interface{}
	err := json.Unmarshal([]byte(v.StringValue.ValueString()), &priorJson)
	if err != nil {
		diags.AddError(
			"Semantic Equality Check Error",
			"An error occurred while normalizing the prior JSON string. "+
				"Please report this to the provider developers.\n\n"+
				"Error: "+err.Error(),
		)

		return false, diags
	}

	var newJson interface{}
	err = json.Unmarshal([]byte(newValue.ValueString()), &newJson)
	if err != nil {
		diags.AddError(
			"Semantic Equality Check Error",
			"An error occurred while normalizing the new JSON string. "+
				"Please report this to the provider developers.\n\n"+
				"Error: "+err.Error(),
		)

		return false, diags
	}

	// If the times are equivalent, keep the prior value
	return cmp.Equal(priorJson, newJson), diags
}

func (v JsonStringValue) validate(value string) error {
	var j interface{}
	err := json.Unmarshal([]byte(value), &j)
	return err
}

// Implementation of the xattr.ValidateableAttribute interface
func (v JsonStringValue) ValidateAttribute(ctx context.Context, req xattr.ValidateAttributeRequest, resp *xattr.ValidateAttributeResponse) {
	if v.IsNull() || v.IsUnknown() {
		return
	}

	err := v.validate(v.ValueString())

	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON String Value",
			"An unexpected error occurred while converting a string value that was expected to be JSON string format. "+
				"The string should be a valid JSON string\n\n"+
				"Path: "+req.Path.String()+"\n"+
				"Given Value: "+v.ValueString()+"\n"+
				"Error: "+err.Error(),
		)

		return
	}
}
