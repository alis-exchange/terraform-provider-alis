package spanner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pb "go.protobuf.mentenova.exchange/mentenova/db/resources/bigtable/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal/provider"
)

const (
	DatabaseDialect_GoogleStandardSQL = "GOOGLE_STANDARD_SQL"
	DatabaseDialect_PostgreSQL        = "POSTGRESQL"

	DatabaseState_Creating        = "CREATING"
	DatabaseState_Ready           = "READY"
	DatabaseState_ReadyOptimizing = "READY_OPTIMIZING"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &spannerDatabaseResource{}
	_ resource.ResourceWithConfigure   = &spannerDatabaseResource{}
	_ resource.ResourceWithImportState = &spannerDatabaseResource{}
)

// NewSpannerDatabaseResource is a helper function to simplify the provider implementation.
func NewSpannerDatabaseResource() resource.Resource {
	return &spannerDatabaseResource{}
}

type spannerDatabaseResource struct {
	client pb.SpannerServiceClient
}

type spannerDatabaseModel struct {
	Name                   types.String                    `tfsdk:"name"`
	Project                types.String                    `tfsdk:"project"`
	InstanceName           types.String                    `tfsdk:"instance_name"`
	Dialect                types.String                    `tfsdk:"dialect"`
	EnableDropProtection   types.Bool                      `tfsdk:"enable_drop_protection"`
	EncryptionConfig       spannerDatabaseEncryptionConfig `tfsdk:"encryption_config"`
	VersionRetentionPeriod types.String                    `tfsdk:"version_retention_period"`

	// Computed
	State               types.String                    `tfsdk:"state"`
	CreateTime          types.String                    `tfsdk:"create_time"`
	EarliestVersionTime types.String                    `tfsdk:"earliest_version_time"`
	EncryptionInfo      []spannerDatabaseEncryptionInfo `tfsdk:"encryption_info"`
	DefaultLeader       types.String                    `tfsdk:"default_leader"`
	Reconciling         types.Bool                      `tfsdk:"reconciling"`
}

type spannerDatabaseEncryptionConfig struct {
	KmsKeyName types.String `tfsdk:"kms_key_name"`
}

type spannerDatabaseEncryptionInfo struct {
	KmsKeyVersion types.String `tfsdk:"kms_key_version"`
}

// Metadata returns the resource type name.
func (r *spannerDatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spanner_database"
}

// Schema defines the schema for the resource.
func (r *spannerDatabaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
			"project": schema.StringAttribute{
				Required: true,
			},
			"instance_name": schema.StringAttribute{
				Required: true,
			},
			"dialect": schema.StringAttribute{
				Optional: true,
			},
			"enable_drop_protection": schema.BoolAttribute{
				Optional: true,
			},
			"encryption_config": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"kms_key_name": schema.StringAttribute{
						Required: true,
					},
				},
			},
			"version_retention_period": schema.StringAttribute{
				Optional: true,
			},
			"state": schema.StringAttribute{
				Computed: true,
			},
			"create_time": schema.StringAttribute{
				Computed: true,
			},
			"earliest_version_time": schema.StringAttribute{
				Computed: true,
			},
			"encryption_info": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"kms_key_version": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
			"default_leader": schema.StringAttribute{
				Computed: true,
			},
			"reconciling": schema.BoolAttribute{
				Computed: true,
			},
		},
	}
}

// Create a new resource.
func (r *spannerDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan spannerDatabaseModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate table from plan
	database := &pb.Database{}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.InstanceName.ValueString()
	databaseId := plan.Name.ValueString()

	// Populate deletion protection if any
	if !plan.EnableDropProtection.IsNull() {
		database.EnableDropProtection = plan.EnableDropProtection.ValueBool()
	} else {
		database.EnableDropProtection = false
	}

	// Populate dialect if any
	if !plan.Dialect.IsNull() {
		switch plan.Dialect.ValueString() {
		case DatabaseDialect_GoogleStandardSQL:
			database.Dialect = pb.Database_GOOGLE_STANDARD_SQL
		case DatabaseDialect_PostgreSQL:
			database.Dialect = pb.Database_POSTGRESQL
		}
	}

	// Populate version retention period if any
	if !plan.VersionRetentionPeriod.IsNull() {
		duration, err := time.ParseDuration(plan.VersionRetentionPeriod.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Parsing Version Retention Period",
				"Could not parse Version Retention Period: "+err.Error(),
			)
			return
		}
		database.VersionRetentionPeriod = durationpb.New(duration)
	}

	// Populate encryption config if any
	if !plan.EncryptionConfig.KmsKeyName.IsNull() {
		database.EncryptionConfig = &pb.Database_EncryptionConfig{
			KmsKeyName: plan.EncryptionConfig.KmsKeyName.ValueString(),
		}
	}

	// Create table
	_, err := r.client.CreateDatabase(ctx, &pb.CreateDatabaseRequest{
		Parent:     fmt.Sprintf("projects/%s/instances/%s", project, instanceName),
		DatabaseId: databaseId,
		Database:   database,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Database",
			"Could not create Database ("+databaseId+") in project ("+project+") and instance ("+instanceName+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(databaseId)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *spannerDatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state spannerDatabaseModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.InstanceName.ValueString()
	databaseId := state.Name.ValueString()

	// Get database from API
	database, err := r.client.GetDatabase(ctx, &pb.GetDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instanceName, databaseId),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Database",
			"Could not read Database ("+state.Name.ValueString()+"): "+err.Error(),
		)
		return
	}

	// Set refreshed state
	state.Name = types.StringValue(databaseId)

	// Populate dialect
	switch database.GetDialect() {
	case pb.Database_GOOGLE_STANDARD_SQL:
		state.Dialect = types.StringValue(DatabaseDialect_GoogleStandardSQL)
	case pb.Database_POSTGRESQL:
		state.Dialect = types.StringValue(DatabaseDialect_PostgreSQL)
	}

	// Populate drop protection
	state.EnableDropProtection = types.BoolValue(database.GetEnableDropProtection())

	// Populate encryption config
	if database.GetEncryptionConfig() != nil {
		state.EncryptionConfig = spannerDatabaseEncryptionConfig{
			KmsKeyName: types.StringValue(database.GetEncryptionConfig().GetKmsKeyName()),
		}
	}

	// Populate version retention period
	if database.GetVersionRetentionPeriod() != nil {
		state.VersionRetentionPeriod = types.StringValue(database.GetVersionRetentionPeriod().AsDuration().String())
	}

	// Populate state
	switch database.GetState() {
	case pb.Database_CREATING:
		state.State = types.StringValue(DatabaseState_Creating)
	case pb.Database_READY:
		state.State = types.StringValue(DatabaseState_Ready)
	case pb.Database_READY_OPTIMIZING:
		state.State = types.StringValue(DatabaseState_ReadyOptimizing)
	}

	// Populate create time
	if database.GetCreateTime() != nil {
		state.CreateTime = types.StringValue(database.GetCreateTime().AsTime().Format(time.RFC3339))
	}

	// Populate earliest version time
	if database.GetEarliestVersionTime() != nil {
		state.EarliestVersionTime = types.StringValue(database.GetEarliestVersionTime().AsTime().Format(time.RFC3339))
	}

	// Populate encryption info
	if database.GetEncryptionInfo() != nil && len(database.GetEncryptionInfo()) > 0 {
		for _, encryptionInfo := range database.GetEncryptionInfo() {
			state.EncryptionInfo = append(state.EncryptionInfo, spannerDatabaseEncryptionInfo{
				KmsKeyVersion: types.StringValue(encryptionInfo.GetKmsKeyVersion()),
			})
		}
	}

	// Populate default leader
	if database.GetDefaultLeader() != "" {
		state.DefaultLeader = types.StringValue(database.GetDefaultLeader())
	}

	// Populate reconciling
	state.Reconciling = types.BoolValue(database.GetReconciling())

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *spannerDatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan spannerDatabaseModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.InstanceName.ValueString()
	databaseId := plan.Name.ValueString()

	// Generate database from plan
	database := &pb.Database{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instanceName, databaseId),
	}

	// Populate drop protection if any
	if !plan.EnableDropProtection.IsNull() {
		database.EnableDropProtection = plan.EnableDropProtection.ValueBool()
	}

	// Populate dialect if any
	if !plan.Dialect.IsNull() {
		switch plan.Dialect.ValueString() {
		case DatabaseDialect_GoogleStandardSQL:
			database.Dialect = pb.Database_GOOGLE_STANDARD_SQL
		case DatabaseDialect_PostgreSQL:
			database.Dialect = pb.Database_POSTGRESQL
		}
	}

	// Populate version retention period if any
	if !plan.VersionRetentionPeriod.IsNull() {
		duration, err := time.ParseDuration(plan.VersionRetentionPeriod.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Parsing Version Retention Period",
				"Could not parse Version Retention Period: "+err.Error(),
			)
			return
		}
		database.VersionRetentionPeriod = durationpb.New(duration)
	}

	// Populate encryption config if any
	if !plan.EncryptionConfig.KmsKeyName.IsNull() {
		database.EncryptionConfig = &pb.Database_EncryptionConfig{
			KmsKeyName: plan.EncryptionConfig.KmsKeyName.ValueString(),
		}
	}

	// Update existing table
	_, err := r.client.UpdateDatabase(ctx, &pb.UpdateDatabaseRequest{
		Database: database,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"enable_drop_protection", "dialect", "version_retention_period", "encryption_config"},
		},
		AllowMissing: true,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Database",
			"Could not update Database ("+databaseId+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(databaseId)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *spannerDatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state spannerDatabaseModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.InstanceName.ValueString()
	databaseId := state.Name.ValueString()

	// Delete existing database
	_, err := r.client.DeleteDatabase(ctx, &pb.DeleteDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instanceName, databaseId),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Database",
			"Could not delete Database ("+state.Name.ValueString()+"): "+err.Error(),
		)
		return
	}
}

func (r *spannerDatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Split import ID to get project, instance, and database id
	// projects/{project}/instances/{instance}/databases/{table}
	importIDParts := strings.Split(req.ID, "/")
	if len(importIDParts) != 6 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format projects/{project}/instances/{instance}/databases/{table}",
		)
	}
	project := importIDParts[1]
	instanceName := importIDParts[3]
	databaseName := importIDParts[5]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance_name"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), databaseName)...)
}

// Configure adds the provider configured client to the resource.
func (r *spannerDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(provider.ProviderClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected ProviderClients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = clients.Spanner
}

func (r *spannerDatabaseResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{

		resourcevalidator.Conflicting(),
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
