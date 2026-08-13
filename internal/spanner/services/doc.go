// Package services implements the orchestration layer between the Terraform
// resources and Spanner: each SpannerService method validates its arguments,
// builds DDL through the schema package, and executes it via conn.Connection.
//
// Methods return gRPC status errors — codes.InvalidArgument for malformed
// names or missing fields, codes.NotFound for absent objects — which the
// resource layer maps to diagnostics or state removal on refresh.
package services
