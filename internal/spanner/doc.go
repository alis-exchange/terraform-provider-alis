// Package spanner implements the Terraform resources and data sources the
// provider exposes for Google Cloud Spanner schema objects: tables, secondary
// indexes, foreign key constraints, row deletion (TTL) policies, sequences,
// database roles, and table IAM bindings.
//
// Each resource translates between its Terraform plan/state model and the
// structs of the schema package, then delegates all Spanner work to the
// services.SpannerService shared through the provider configuration (see
// configureProviderConfig). Resource identities follow the
// projects/{project}/instances/{instance}/databases/{database}/... convention
// handled by the names package.
package spanner
