package spanner

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/function"
)

// Ensure the implementations satisfy the expected interfaces.
var (
	_ function.Function = &protoTimestampDdlFunction{}
	_ function.Function = &resourceNameAncestorDdlFunction{}
	_ function.Function = &resourceNameIDDdlFunction{}
)

// The generated expressions are compared verbatim against
// INFORMATION_SCHEMA.COLUMNS.GENERATION_EXPRESSION on refresh, so their exact
// formatting (raw-string regex literals, no space after the TIMESTAMP_ADD
// comma) is load-bearing: Spanner stores the text as written, and any
// formatting drift would surface as a permanent plan diff on the column.

// identifierPathRegex accepts a dotted SQL identifier path such as
// Book.create_time, guarding the generated DDL against SQL injection.
var identifierPathRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// collectionIDRegex accepts an AIP-122 collection identifier such as
// shelves or books.
var collectionIDRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

func validateFieldPath(fieldPath string) error {
	if !identifierPathRegex.MatchString(fieldPath) {
		return fmt.Errorf(
			"field_path (%s) must be a dotted identifier path of letters, digits and underscores, e.g. Book.create_time",
			fieldPath,
		)
	}
	return nil
}

func validateCollectionID(collection string) error {
	if !collectionIDRegex.MatchString(collection) {
		return fmt.Errorf("collection (%s) must be a collection identifier of letters, digits and underscores, e.g. shelves", collection)
	}
	return nil
}

// protoTimestampDdl builds the computed-column expression converting a
// google.protobuf.Timestamp field into a Spanner TIMESTAMP at microsecond
// precision.
func protoTimestampDdl(fieldPath string) (string, error) {
	if err := validateFieldPath(fieldPath); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"TIMESTAMP_ADD(TIMESTAMP_SECONDS(%s.seconds),INTERVAL CAST(FLOOR(%s.nanos / 1000) AS INT64) MICROSECOND)",
		fieldPath, fieldPath,
	), nil
}

// resourceNameAncestorDdl builds a REGEXP_EXTRACT expression returning the
// ancestor prefix of an AIP-122 resource name up to and including the ID of
// the last given collection, e.g. shelves/123 out of
// shelves/123/books/456.
func resourceNameAncestorDdl(fieldPath string, collections []string) (string, error) {
	if err := validateFieldPath(fieldPath); err != nil {
		return "", err
	}
	if len(collections) == 0 {
		return "", errors.New("at least one collection identifier is required, e.g. shelves")
	}
	segments := make([]string, 0, len(collections))
	for _, collection := range collections {
		if err := validateCollectionID(collection); err != nil {
			return "", err
		}
		segments = append(segments, collection+"/[^/]+")
	}
	return fmt.Sprintf("REGEXP_EXTRACT(%s, r'^(%s)')", fieldPath, strings.Join(segments, "/")), nil
}

// resourceNameIDDdl builds a REGEXP_EXTRACT expression returning the resource
// ID that follows a collection segment in an AIP-122 resource name, e.g. 456
// out of shelves/123/books/456 for collection
// books.
func resourceNameIDDdl(fieldPath, collection string) (string, error) {
	if err := validateFieldPath(fieldPath); err != nil {
		return "", err
	}
	if err := validateCollectionID(collection); err != nil {
		return "", err
	}
	return fmt.Sprintf("REGEXP_EXTRACT(%s, r'%s/([^/]+)')", fieldPath, collection), nil
}

// NewProtoTimestampDdlFunction is a helper function to simplify the provider implementation.
func NewProtoTimestampDdlFunction() function.Function {
	return &protoTimestampDdlFunction{}
}

type protoTimestampDdlFunction struct{}

// Metadata returns the function name.
func (f *protoTimestampDdlFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "proto_timestamp_ddl"
}

// Definition returns the function signature and documentation.
func (f *protoTimestampDdlFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Generates computed-column DDL converting a proto Timestamp field to a Spanner TIMESTAMP.",
		MarkdownDescription: "Generates a `computation_ddl` expression for `alis_google_spanner_table` computed columns that converts a " +
			"`google.protobuf.Timestamp` field of a `PROTO` column into a Spanner `TIMESTAMP` at microsecond precision.\n\n" +
			"For example `provider::alis::proto_timestamp_ddl(\"Book.create_time\")` returns\n" +
			"`TIMESTAMP_ADD(TIMESTAMP_SECONDS(Book.create_time.seconds),INTERVAL CAST(FLOOR(Book.create_time.nanos / 1000) AS INT64) MICROSECOND)`.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name: "field_path",
				MarkdownDescription: "Dotted path to the `google.protobuf.Timestamp` field, starting at the proto column name, " +
					"e.g. `Book.create_time`.",
			},
		},
		Return: function.StringReturn{},
	}
}

// Run generates the DDL expression.
func (f *protoTimestampDdlFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var fieldPath string
	resp.Error = function.ConcatFuncErrors(resp.Error, req.Arguments.Get(ctx, &fieldPath))
	if resp.Error != nil {
		return
	}

	ddl, err := protoTimestampDdl(fieldPath)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewArgumentFuncError(0, err.Error()))
		return
	}
	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, ddl))
}

// NewResourceNameAncestorDdlFunction is a helper function to simplify the provider implementation.
func NewResourceNameAncestorDdlFunction() function.Function {
	return &resourceNameAncestorDdlFunction{}
}

type resourceNameAncestorDdlFunction struct{}

// Metadata returns the function name.
func (f *resourceNameAncestorDdlFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "resource_name_ancestor_ddl"
}

// Definition returns the function signature and documentation.
func (f *resourceNameAncestorDdlFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Generates computed-column DDL extracting an ancestor prefix from an AIP-122 resource name.",
		MarkdownDescription: "Generates a `computation_ddl` expression for `alis_google_spanner_table` computed columns that extracts the " +
			"ancestor prefix of an [AIP-122](https://google.aip.dev/122) resource name, up to and including the ID of the last given collection. " +
			"The generated `REGEXP_EXTRACT` always has exactly one capturing group, which Spanner requires.\n\n" +
			"For example `provider::alis::resource_name_ancestor_ddl(\"Book.name\", \"shelves\")` returns\n" +
			"`REGEXP_EXTRACT(Book.name, r'^(shelves/[^/]+)')`, extracting `shelves/123` from `shelves/123/books/456`.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name: "field_path",
				MarkdownDescription: "Dotted path to the resource-name field, starting at the column name, " +
					"e.g. `Book.name` or a plain `STRING` column name.",
			},
		},
		VariadicParameter: function.StringParameter{
			Name: "collections",
			MarkdownDescription: "Collection identifiers from the root of the resource name, in order, e.g. `\"shelves\"` " +
				"or `\"shelves\", \"books\"`. At least one is required.",
		},
		Return: function.StringReturn{},
	}
}

// Run generates the DDL expression.
func (f *resourceNameAncestorDdlFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var fieldPath string
	var collections []string
	resp.Error = function.ConcatFuncErrors(resp.Error, req.Arguments.Get(ctx, &fieldPath, &collections))
	if resp.Error != nil {
		return
	}

	ddl, err := resourceNameAncestorDdl(fieldPath, collections)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(err.Error()))
		return
	}
	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, ddl))
}

// NewResourceNameIDDdlFunction is a helper function to simplify the provider implementation.
func NewResourceNameIDDdlFunction() function.Function {
	return &resourceNameIDDdlFunction{}
}

type resourceNameIDDdlFunction struct{}

// Metadata returns the function name.
func (f *resourceNameIDDdlFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "resource_name_id_ddl"
}

// Definition returns the function signature and documentation.
func (f *resourceNameIDDdlFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Generates computed-column DDL extracting a resource ID from an AIP-122 resource name.",
		MarkdownDescription: "Generates a `computation_ddl` expression for `alis_google_spanner_table` computed columns that extracts the " +
			"resource ID following a collection segment in an [AIP-122](https://google.aip.dev/122) resource name. " +
			"The generated `REGEXP_EXTRACT` always has exactly one capturing group, which Spanner requires.\n\n" +
			"For example `provider::alis::resource_name_id_ddl(\"Book.name\", \"books\")` returns\n" +
			"`REGEXP_EXTRACT(Book.name, r'books/([^/]+)')`, extracting `456` from `shelves/123/books/456`.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name: "field_path",
				MarkdownDescription: "Dotted path to the resource-name field, starting at the column name, " +
					"e.g. `Book.name` or a plain `STRING` column name.",
			},
			function.StringParameter{
				Name:                "collection",
				MarkdownDescription: "The collection identifier immediately preceding the ID to extract, e.g. `books`.",
			},
		},
		Return: function.StringReturn{},
	}
}

// Run generates the DDL expression.
func (f *resourceNameIDDdlFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var fieldPath, collection string
	resp.Error = function.ConcatFuncErrors(resp.Error, req.Arguments.Get(ctx, &fieldPath, &collection))
	if resp.Error != nil {
		return
	}

	ddl, err := resourceNameIDDdl(fieldPath, collection)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(err.Error()))
		return
	}
	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, ddl))
}
