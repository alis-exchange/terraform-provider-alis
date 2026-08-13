// Package schema models Google Spanner schema objects — tables, columns,
// indexes, foreign-key constraints, row deletion (TTL) policies, database
// roles and grants, and sequences — and renders them as GoogleSQL DDL.
//
// The DDL builders (CreateDdl, AlterDdl, and the Drop*Ddl helpers) are pure:
// struct in, DDL string out, no IO. SpannerTable is the exception on both
// sides of that boundary: Get hydrates a table from INFORMATION_SCHEMA
// through a conn.Connection, and Create, Update, and Delete submit the
// generated DDL through the same connection.
package schema
