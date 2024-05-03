package spanner

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"terraform-provider-alis/internal"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"
	"terraform-provider-alis/internal/validators"
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
	config *internal.ProviderConfig
}

type spannerDatabaseModel struct {
	Name                   types.String                     `tfsdk:"name"`
	Project                types.String                     `tfsdk:"project"`
	Instance               types.String                     `tfsdk:"instance"`
	Dialect                types.String                     `tfsdk:"dialect"`
	EnableDropProtection   types.Bool                       `tfsdk:"enable_drop_protection"`
	EncryptionConfig       *spannerDatabaseEncryptionConfig `tfsdk:"encryption_config"`
	VersionRetentionPeriod types.String                     `tfsdk:"version_retention_period"`

	// Computed
	State               types.String `tfsdk:"state"`
	CreateTime          types.String `tfsdk:"create_time"`
	EarliestVersionTime types.String `tfsdk:"earliest_version_time"`
	EncryptionInfo      types.List   `tfsdk:"encryption_info"`
	DefaultLeader       types.String `tfsdk:"default_leader"`
	Reconciling         types.Bool   `tfsdk:"reconciling"`
}

type spannerDatabaseEncryptionConfig struct {
	KmsKeyName types.String `tfsdk:"kms_key_name"`
}

type spannerDatabaseEncryptionInfo struct {
	// TODO: Add status
	Type          types.String `tfsdk:"type"`
	KmsKeyVersion types.String `tfsdk:"kms_key_version"`
}

func (o spannerDatabaseEncryptionInfo) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":            types.StringType,
		"kms_key_version": types.StringType,
	}
}

// Metadata returns the resource type name.
func (r *spannerDatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_database"
}

// Schema defines the schema for the resource.
func (r *spannerDatabaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z][a-z0-9_\-]*[a-z0-9]{2,30}$`), "Name must be a valid Spanner Database ID"),
					validators.RegexMatches([]*regexp.Regexp{
						regexp.MustCompile(utils.SpannerGoogleSqlDatabaseIdRegex),
						regexp.MustCompile(utils.SpannerPostgresSqlDatabaseIdRegex),
					}, "Name must be a valid Spanner Database ID"),
					stringvalidator.LengthBetween(2, 30),
				},
				Description: "A unique identifier for the database, which cannot be changed after\n" +
					"the instance is created. Values are of the form `[a-z][-a-z0-9]*[a-z0-9]` and must be 2-30 characters long.",
			},
			"project": schema.StringAttribute{
				Required:    true,
				Description: "The Google Cloud project ID.",
			},
			"instance": schema.StringAttribute{
				Required:    true,
				Description: "The Spanner instance ID.",
			},
			"dialect": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(services.DatabaseDialect_GoogleStandardSQL, services.DatabaseDialect_PostgreSQL),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "The dialect of the Cloud Spanner Database.\n" +
					"If it is not provided, `GOOGLE_STANDARD_SQL` will be used. Possible values: [`GOOGLE_STANDARD_SQL`, `POSTGRESQL`]",
			},
			"enable_drop_protection": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether drop protection is enabled for this database. Defaults to false.",
			},
			"encryption_config": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"kms_key_name": schema.StringAttribute{
						Required: true,
						Description: "Fully qualified name of the KMS key to use to encrypt this database. This key must exist\n" +
							"in the same location as the Spanner Database.",
					},
				},
				Description: "Encryption configuration for the database",
			},
			"version_retention_period": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[1-9][0-9]*s$`), "Version Retention Period must be a valid duration specified in seconds in the format `{seconds}s` e.g. `60s`"),
					validators.DurationStringMinSeconds(60 * 60),
					validators.DurationStringMaxSeconds(60 * 60 * 24 * 7),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description: "The retention period for the database. The retention period must be between 1 hour\n" +
					"and 7 days, and must be specified in seconds. For example, 86400s is equivalent to 1 day.",
			},
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "An explanation of the status of the database.",
			},
			"create_time": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "The time at which the database was created.",
			},
			"earliest_version_time": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"encryption_info": schema.ListNestedAttribute{
				Computed: true,
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"kms_key_version": schema.StringAttribute{
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"default_leader": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"reconciling": schema.BoolAttribute{
				Computed: true,
			},
		},
		Description: "A Cloud Spanner Database resource.\n" +
			"This resource provisions and manages Cloud Spanner Databases.",
	}
}

// Create a new resource.
func (r *spannerDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan spannerDatabaseModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
		return
	}

	// Generate database from plan
	database := &databasepb.Database{}

	// Get project and instance name
	project := plan.Project.ValueString()
	instanceName := plan.Instance.ValueString()
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
		case services.DatabaseDialect_GoogleStandardSQL:
			database.DatabaseDialect = databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL
		case services.DatabaseDialect_PostgreSQL:
			database.DatabaseDialect = databasepb.DatabaseDialect_POSTGRESQL
		}
	}

	// Populate version retention period if any
	if !plan.VersionRetentionPeriod.IsNull() {
		database.VersionRetentionPeriod = plan.VersionRetentionPeriod.ValueString()
	}

	// Populate encryption config if any
	if plan.EncryptionConfig != nil && !plan.EncryptionConfig.KmsKeyName.IsNull() {
		database.EncryptionConfig = &databasepb.EncryptionConfig{
			KmsKeyName: plan.EncryptionConfig.KmsKeyName.ValueString(),
		}
	}

	// Create table
	database, err := r.config.SpannerService.CreateSpannerDatabase(ctx,
		fmt.Sprintf("projects/%s/instances/%s", project, instanceName),
		databaseId,
		database,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Database",
			"Could not create Database ("+databaseId+") in project ("+project+") and instance ("+instanceName+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(databaseId)

	// Populate state
	switch database.GetState() {
	case databasepb.Database_CREATING:
		plan.State = types.StringValue(services.DatabaseState_Creating)
	case databasepb.Database_READY:
		plan.State = types.StringValue(services.DatabaseState_Ready)
	case databasepb.Database_READY_OPTIMIZING:
		plan.State = types.StringValue(services.DatabaseState_ReadyOptimizing)
	default:
		plan.State = types.StringNull()
	}

	// Populate create time
	if database.GetCreateTime() != nil {
		plan.CreateTime = types.StringValue(database.GetCreateTime().AsTime().Format(time.RFC3339))
	} else {
		plan.CreateTime = types.StringValue("")
	}

	// Populate earliest version time
	if database.GetEarliestVersionTime() != nil {
		plan.EarliestVersionTime = types.StringValue(database.GetEarliestVersionTime().AsTime().Format(time.RFC3339))
	} else {
		plan.EarliestVersionTime = types.StringValue("")
	}

	// Populate encryption info
	var encryptionInfoList []spannerDatabaseEncryptionInfo
	if database.GetEncryptionInfo() != nil && len(database.GetEncryptionInfo()) > 0 {
		for _, encryptionInfo := range database.GetEncryptionInfo() {
			var encryptionType types.String
			switch encryptionInfo.GetEncryptionType() {
			case databasepb.EncryptionInfo_CUSTOMER_MANAGED_ENCRYPTION:
				encryptionType = types.StringValue(services.DatabaseEncryptionType_CustomerManaged)
			case databasepb.EncryptionInfo_GOOGLE_DEFAULT_ENCRYPTION:
				encryptionType = types.StringValue(services.DatabaseEncryptionType_GoogleDefaultEncryption)
			default:
				encryptionType = types.StringNull()
			}
			encryptionInfoList = append(encryptionInfoList, spannerDatabaseEncryptionInfo{
				Type:          encryptionType,
				KmsKeyVersion: types.StringValue(encryptionInfo.GetKmsKeyVersion()),
			})
		}
	} else {
		encryptionInfoList = []spannerDatabaseEncryptionInfo{}
	}
	generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: spannerDatabaseEncryptionInfo{}.attrTypes(),
	}, []spannerDatabaseEncryptionInfo{})
	diags.Append(d...)
	plan.EncryptionInfo = generatedList

	// Populate default leader
	if database.GetDefaultLeader() != "" {
		plan.DefaultLeader = types.StringValue(database.GetDefaultLeader())
	} else {
		plan.DefaultLeader = types.StringValue("")
	}

	// Populate reconciling
	plan.Reconciling = types.BoolValue(database.GetReconciling())

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
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
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
		return
	}

	// Get project and instance name
	project := state.Project.ValueString()
	instanceName := state.Instance.ValueString()
	databaseId := state.Name.ValueString()

	// Get database from API
	database, err := r.config.SpannerService.GetSpannerDatabase(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instanceName, databaseId),
	)
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
	switch database.GetDatabaseDialect() {
	case databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL:
		state.Dialect = types.StringValue(services.DatabaseDialect_GoogleStandardSQL)
	case databasepb.DatabaseDialect_POSTGRESQL:
		state.Dialect = types.StringValue(services.DatabaseDialect_PostgreSQL)
	}

	// Populate drop protection
	state.EnableDropProtection = types.BoolValue(database.GetEnableDropProtection())

	// Populate encryption config
	if database.GetEncryptionConfig() != nil {
		state.EncryptionConfig = &spannerDatabaseEncryptionConfig{
			KmsKeyName: types.StringValue(database.GetEncryptionConfig().GetKmsKeyName()),
		}
	}

	// Populate version retention period
	if database.GetVersionRetentionPeriod() != "" {
		state.VersionRetentionPeriod = types.StringValue(database.GetVersionRetentionPeriod())
	}

	// Populate state
	switch database.GetState() {
	case databasepb.Database_CREATING:
		state.State = types.StringValue(services.DatabaseState_Creating)
	case databasepb.Database_READY:
		state.State = types.StringValue(services.DatabaseState_Ready)
	case databasepb.Database_READY_OPTIMIZING:
		state.State = types.StringValue(services.DatabaseState_ReadyOptimizing)
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

		var encryptionInfoList []spannerDatabaseEncryptionInfo
		for _, encryptionInfo := range database.GetEncryptionInfo() {
			var encryptionType types.String
			switch encryptionInfo.GetEncryptionType() {
			case databasepb.EncryptionInfo_CUSTOMER_MANAGED_ENCRYPTION:
				encryptionType = types.StringValue(services.DatabaseEncryptionType_CustomerManaged)
			case databasepb.EncryptionInfo_GOOGLE_DEFAULT_ENCRYPTION:
				encryptionType = types.StringValue(services.DatabaseEncryptionType_GoogleDefaultEncryption)
			default:
				encryptionType = types.StringNull()
			}
			encryptionInfoList = append(encryptionInfoList, spannerDatabaseEncryptionInfo{
				Type:          encryptionType,
				KmsKeyVersion: types.StringValue(encryptionInfo.GetKmsKeyVersion()),
			})
		}

		generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: spannerDatabaseEncryptionInfo{}.attrTypes(),
		}, encryptionInfoList)
		diags.Append(d...)

		state.EncryptionInfo = generatedList
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
		tflog.Error(ctx,
			fmt.Sprintf("Error reading state: %v", resp.Diagnostics),
		)
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
	instanceName := plan.Instance.ValueString()
	databaseId := plan.Name.ValueString()

	// Generate database from plan
	database := &databasepb.Database{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instanceName, databaseId),
	}

	// Populate drop protection if any
	if !plan.EnableDropProtection.IsNull() {
		database.EnableDropProtection = plan.EnableDropProtection.ValueBool()
	}

	// Populate dialect if any
	if !plan.Dialect.IsNull() {
		switch plan.Dialect.ValueString() {
		case services.DatabaseDialect_GoogleStandardSQL:
			database.DatabaseDialect = databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL
		case services.DatabaseDialect_PostgreSQL:
			database.DatabaseDialect = databasepb.DatabaseDialect_POSTGRESQL
		}
	}

	// Populate version retention period if any
	if !plan.VersionRetentionPeriod.IsNull() {
		database.VersionRetentionPeriod = plan.VersionRetentionPeriod.ValueString()
	}

	// Populate encryption config if any
	if plan.EncryptionConfig != nil && !plan.EncryptionConfig.KmsKeyName.IsNull() {
		database.EncryptionConfig = &databasepb.EncryptionConfig{
			KmsKeyName: plan.EncryptionConfig.KmsKeyName.ValueString(),
		}
	}

	// Update existing table
	_, err := r.config.SpannerService.UpdateSpannerDatabase(ctx,
		database,
		&fieldmaskpb.FieldMask{
			Paths: []string{"enable_drop_protection", "dialect", "version_retention_period", "encryption_config"},
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Database",
			"Could not update Database ("+databaseId+"): "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(databaseId)
	// Populate state
	switch database.GetState() {
	case databasepb.Database_CREATING:
		plan.State = types.StringValue(services.DatabaseState_Creating)
	case databasepb.Database_READY:
		plan.State = types.StringValue(services.DatabaseState_Ready)
	case databasepb.Database_READY_OPTIMIZING:
		plan.State = types.StringValue(services.DatabaseState_ReadyOptimizing)
	default:
		plan.State = types.StringNull()
	}
	// Populate create time
	if database.GetCreateTime() != nil {
		plan.CreateTime = types.StringValue(database.GetCreateTime().AsTime().Format(time.RFC3339))
	} else {
		plan.CreateTime = types.StringValue("")
	}
	// Populate earliest version time
	if database.GetEarliestVersionTime() != nil {
		plan.EarliestVersionTime = types.StringValue(database.GetEarliestVersionTime().AsTime().Format(time.RFC3339))
	} else {
		plan.EarliestVersionTime = types.StringValue("")
	}
	var encryptionInfoList []spannerDatabaseEncryptionInfo
	if database.GetEncryptionInfo() != nil && len(database.GetEncryptionInfo()) > 0 {
		for _, encryptionInfo := range database.GetEncryptionInfo() {
			var encryptionType types.String
			switch encryptionInfo.GetEncryptionType() {
			case databasepb.EncryptionInfo_CUSTOMER_MANAGED_ENCRYPTION:
				encryptionType = types.StringValue(services.DatabaseEncryptionType_CustomerManaged)
			case databasepb.EncryptionInfo_GOOGLE_DEFAULT_ENCRYPTION:
				encryptionType = types.StringValue(services.DatabaseEncryptionType_GoogleDefaultEncryption)
			default:
				encryptionType = types.StringNull()
			}
			encryptionInfoList = append(encryptionInfoList, spannerDatabaseEncryptionInfo{
				Type:          encryptionType,
				KmsKeyVersion: types.StringValue(encryptionInfo.GetKmsKeyVersion()),
			})
		}

		generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: spannerDatabaseEncryptionInfo{}.attrTypes(),
		}, encryptionInfoList)
		diags.Append(d...)

		plan.EncryptionInfo = generatedList
	} else {
		encryptionInfoList = []spannerDatabaseEncryptionInfo{}
	}
	generatedList, d := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: spannerDatabaseEncryptionInfo{}.attrTypes(),
	}, []spannerDatabaseEncryptionInfo{})
	diags.Append(d...)
	plan.EncryptionInfo = generatedList
	// Populate default leader
	if database.GetDefaultLeader() != "" {
		plan.DefaultLeader = types.StringValue(database.GetDefaultLeader())
	} else {
		plan.DefaultLeader = types.StringValue("")
	}
	// Populate reconciling
	plan.Reconciling = types.BoolValue(database.GetReconciling())

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
	instanceName := state.Instance.ValueString()
	databaseId := state.Name.ValueString()

	// Delete existing database
	_, err := r.config.SpannerService.DeleteSpannerDatabase(ctx,
		fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instanceName, databaseId),
	)
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), instanceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), databaseName)...)
}

// Configure adds the provider configured client to the resource.
func (r *spannerDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*internal.ProviderConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *utils.ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.config = config
}

func (r *spannerDatabaseResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{

		//resourcevalidator.Conflicting(),
		//resourcevalidator.Conflicting(
		//	path.MatchRoot("attribute_one"),
		//	path.MatchRoot("attribute_two"),
		//),
	}
}
