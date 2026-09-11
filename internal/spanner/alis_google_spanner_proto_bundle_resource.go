package spanner

import (
	"context"
	"slices"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/spanner/services"
	"terraform-provider-alis/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                   = &protoBundleResource{}
	_ resource.ResourceWithConfigure      = &protoBundleResource{}
	_ resource.ResourceWithImportState    = &protoBundleResource{}
	_ resource.ResourceWithModifyPlan     = &protoBundleResource{}
	_ resource.ResourceWithValidateConfig = &protoBundleResource{}
)

// NewProtoBundleResource is a helper function to simplify the provider implementation.
func NewProtoBundleResource() resource.Resource {
	return &protoBundleResource{}
}

// protoBundleResource manages one package's slice of a database's proto
// bundle. A database has a single bundle shared by every service storing
// PROTO columns in it, so the resource owns only the types under its
// packages: it inserts missing types (imports included), updates and deletes
// owned ones, and leaves the rest to whoever owns them.
type protoBundleResource struct {
	service *services.SpannerService
}

type protoBundleModel struct {
	Project           types.String   `tfsdk:"project"`
	Instance          types.String   `tfsdk:"instance"`
	Database          types.String   `tfsdk:"database"`
	Packages          types.Set      `tfsdk:"packages"`
	Sources           types.List     `tfsdk:"sources"`
	DescriptorsSha256 types.String   `tfsdk:"descriptors_sha256"`
	Types             types.Set      `tfsdk:"types"`
	Timeouts          timeouts.Value `tfsdk:"timeouts"`
}

type protoBundleSourceModel struct {
	LocalPath types.String `tfsdk:"local_path"`
	GcsURI    types.String `tfsdk:"gcs_uri"`
}

// Metadata returns the resource type name.
func (r *protoBundleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_spanner_proto_bundle"
}

// Schema defines the schema for the resource.
func (r *protoBundleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Version: resourceSchemaVersion,
		Blocks: map[string]rschema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
		Attributes: map[string]rschema.Attribute{
			"project": rschema.StringAttribute{
				Required: true,
				MarkdownDescription: "The Google Cloud project ID containing the Spanner instance and database.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"instance": rschema.StringAttribute{
				Required: true,
				MarkdownDescription: "The Spanner instance ID that contains the database.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"database": rschema.StringAttribute{
				Required: true,
				MarkdownDescription: "The Spanner database ID whose proto bundle is managed.\n" +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"packages": rschema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Proto packages this resource owns in the bundle, for example `com.example.books.v1`. " +
					"Only messages and enums directly in these packages are updated or deleted; imported types from other packages " +
					"are inserted when missing but otherwise left to their own owner. A nested package (`com.example.books.v1.sub`) is not " +
					"owned by its parent. Removing a package from this set stops managing its types without deleting them.",
				Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
			},
			"sources": rschema.ListNestedAttribute{
				Required: true,
				MarkdownDescription: "Where the FileDescriptorSet comes from. Each entry sets exactly one of `local_path` or `gcs_uri`. " +
					"All sources are merged by proto file name; the same file with different content in two sources is an error. " +
					"The sources are read at plan time, so they must exist wherever `terraform plan` runs.",
				Validators: []validator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"local_path": rschema.StringAttribute{
							Optional: true,
							MarkdownDescription: "A local FileDescriptorSet file, or a directory whose regular files (direct children only) are all read as " +
								"descriptor sets. File names are not filtered: Define writes the set as `fds_including_imports`.",
						},
						"gcs_uri": rschema.StringAttribute{
							Optional: true,
							MarkdownDescription: "A Cloud Storage object (`gs://bucket/path/fds_including_imports`) or prefix (`gs://bucket/path/`, " +
								"direct children only) read with the provider's credentials.",
						},
					},
				},
			},
			"descriptors_sha256": rschema.StringAttribute{
				Computed: true,
				MarkdownDescription: "SHA-256 of the merged descriptor set, computed at plan time. A change in any source plans an in-place " +
					"update that re-sends the owned types' descriptors.",
			},
			"types": rschema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Fully qualified names of the owned messages and enums the bundle holds. Refreshed from the database, " +
					"so a type removed out of band plans an update that re-inserts it.",
			},
		},
		MarkdownDescription: "Manages one package's slice of a Cloud Spanner database's proto bundle from FileDescriptorSet sources. " +
			"A database has a single proto bundle shared by every service that stores PROTO columns in it; this resource creates " +
			"missing types (including imported dependencies), updates and deletes only the types in `packages`, and ignores the rest, " +
			"so several services can manage the same database's bundle independently.\n\n" +
			"Tables with PROTO columns must `depends_on` this resource: Spanner rejects a column whose type is not yet in the bundle, " +
			"and refuses to delete a type a column still references.\n\n" +
			"See https://cloud.google.com/spanner/docs/reference/standard-sql/protocol-buffers",
	}
}

// ValidateConfig enforces exactly one of local_path / gcs_uri per source.
func (r *protoBundleResource) ValidateConfig(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	var config protoBundleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.Sources.IsNull() || config.Sources.IsUnknown() {
		return
	}
	var sources []protoBundleSourceModel
	resp.Diagnostics.Append(config.Sources.ElementsAs(ctx, &sources, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for i, src := range sources {
		if src.LocalPath.IsUnknown() || src.GcsURI.IsUnknown() {
			continue
		}
		hasLocal := !src.LocalPath.IsNull() && src.LocalPath.ValueString() != ""
		hasGcs := !src.GcsURI.IsNull() && src.GcsURI.ValueString() != ""
		if hasLocal == hasGcs {
			resp.Diagnostics.AddAttributeError(
				path.Root("sources").AtListIndex(i),
				"Invalid Descriptor Source",
				"Each sources entry must set exactly one of local_path or gcs_uri.",
			)
		}
	}
}

// ModifyPlan loads the sources and records their hash and owned types in the
// plan. Both are known at plan time whenever the sources are, which is what
// makes a changed fds, a changed package list, or a drifted live type set
// show up as an in-place update. Unknown sources leave both unknown.
func (r *protoBundleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan protoBundleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sources, packages, known, diags := plan.inputs(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !known {
		plan.DescriptorsSha256 = types.StringUnknown()
		plan.Types = types.SetUnknown(types.StringType)
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
		return
	}
	set, err := r.service.LoadDescriptorSources(ctx, sources)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("sources"),
			"Unable to Load Descriptor Sources",
			utils.ErrDetail(err),
		)
		return
	}
	if set.Warning != "" {
		resp.Diagnostics.AddAttributeWarning(path.Root("sources"), "Large Descriptor Set", set.Warning)
	}
	plan.DescriptorsSha256 = types.StringValue(set.Sha256)
	owned := make([]string, 0)
	for name := range schema.OwnedTypes(set.Types, packages) {
		owned = append(owned, name)
	}
	slices.Sort(owned)
	typesValue, diags := types.SetValueFrom(ctx, types.StringType, owned)
	resp.Diagnostics.Append(diags...)
	plan.Types = typesValue
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// inputs extracts the sources and packages from the model. known is false
// when any of them is not yet known (values from other resources).
func (m protoBundleModel) inputs(
	ctx context.Context,
) (sources []schema.DescriptorSource, packages []string, known bool, diags diag.Diagnostics) {
	if m.Sources.IsUnknown() || m.Packages.IsUnknown() {
		return nil, nil, false, nil
	}
	var sourceModels []protoBundleSourceModel
	diags.Append(m.Sources.ElementsAs(ctx, &sourceModels, false)...)
	diags.Append(m.Packages.ElementsAs(ctx, &packages, false)...)
	if diags.HasError() {
		return nil, nil, false, diags
	}
	for _, src := range sourceModels {
		if src.LocalPath.IsUnknown() || src.GcsURI.IsUnknown() {
			return nil, nil, false, diags
		}
		sources = append(sources, schema.DescriptorSource{LocalPath: src.LocalPath.ValueString(), GcsURI: src.GcsURI.ValueString()})
	}
	slices.Sort(packages)
	return sources, packages, true, diags
}

func (m protoBundleModel) databaseName() string {
	return names.DatabaseName{
		Project:  m.Project.ValueString(),
		Instance: m.Instance.ValueString(),
		Database: m.Database.ValueString(),
	}.String()
}

// apply loads the sources and reconciles the bundle, then records the hash
// and the owned types now in the bundle on the model. Shared by Create and
// Update: both are the same reconciliation against whatever is live.
func (r *protoBundleResource) apply(ctx context.Context, plan *protoBundleModel, diags *diag.Diagnostics) {
	sources, packages, known, d := plan.inputs(ctx)
	diags.Append(d...)
	if diags.HasError() {
		return
	}
	if !known {
		diags.AddError("Unknown Descriptor Sources", "sources and packages must be known at apply time.")
		return
	}
	set, err := r.service.LoadDescriptorSources(ctx, sources)
	if err != nil {
		diags.AddAttributeError(path.Root("sources"), "Unable to Load Descriptor Sources", utils.ErrDetail(err))
		return
	}
	database := plan.databaseName()
	owned, ddl, err := r.service.ApplyProtoBundle(ctx, database, set.Data, packages)
	if err != nil {
		detail := "Could not apply proto bundle changes to " + database + ": " + utils.ErrDetail(err)
		if ddl != "" {
			detail += "\nDDL: " + ddl
		}
		diags.AddError("Error Applying Proto Bundle", detail)
		return
	}
	plan.DescriptorsSha256 = types.StringValue(set.Sha256)
	typesValue, d := types.SetValueFrom(ctx, types.StringType, owned)
	diags.Append(d...)
	plan.Types = typesValue
}

// Create reconciles the owned slice into the live bundle. It never adopts
// silently: the DDL is always sent, so a bundle a previous protog run left
// behind is simply brought up to date.
func (r *protoBundleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan protoBundleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, diags := plan.Timeouts.Create(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, createTimeout)
	defer cancel()

	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the owned types from the live bundle. No owned type left
// means the resource is gone. The hash cannot be verified against Spanner
// and is carried over from state.
func (r *protoBundleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state protoBundleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var packages []string
	resp.Diagnostics.Append(state.Packages.ElementsAs(ctx, &packages, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	database := state.databaseName()
	owned, err := r.service.GetProtoBundleTypes(ctx, database, packages)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Proto Bundle",
			"Could not read the proto bundle of "+database+": "+utils.ErrDetail(err),
		)
		return
	}
	typesValue, diags := types.SetValueFrom(ctx, types.StringType, owned)
	resp.Diagnostics.Append(diags...)
	state.Types = typesValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update re-runs the reconciliation; the diff against the live bundle decides
// what is inserted, updated, or deleted.
func (r *protoBundleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan protoBundleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateTimeout, diags := plan.Timeouts.Update(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, updateTimeout)
	defer cancel()

	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the owned types, dropping the bundle when nothing else
// would remain. Types other packages own are untouched.
func (r *protoBundleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state protoBundleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteTimeout, diags := state.Timeouts.Delete(ctx, 0)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := withTimeout(ctx, deleteTimeout)
	defer cancel()

	var packages []string
	resp.Diagnostics.Append(state.Packages.ElementsAs(ctx, &packages, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	database := state.databaseName()
	if err := r.service.DeleteProtoBundle(ctx, database, packages); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Proto Bundle Types",
			"Could not delete the owned proto bundle types from "+database+": "+utils.ErrDetail(err),
		)
	}
}

// ImportState seeds identity and the package from
// projects/{p}/instances/{i}/databases/{d}/protoBundles/{package}; sources
// come from the configuration on the next plan.
func (r *protoBundleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importName, err := names.ParseProtoBundle(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID ("+req.ID+") must be in the format projects/{project}/instances/{instance}/databases/{database}/protoBundles/{package}: "+err.Error(),
		)
		return
	}
	packages, diags := types.SetValueFrom(ctx, types.StringType, []string{importName.Package})
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), importName.Project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), importName.Instance)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), importName.Database)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("packages"), packages)...)
}

// Configure adds the provider configured client to the resource.
func (r *protoBundleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	service, ok := configureSpannerService(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}
	r.service = service
}
