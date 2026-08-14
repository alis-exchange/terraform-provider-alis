# Binding can be imported by specifying the fully qualified name of the table role binding
# projects/{project}/instances/{instance}/databases/{database}/tables/{table}/tableRoles/{role}
terraform import alis_google_spanner_table_iam_binding.binding "projects/{project}/instances/{instance}/databases/{database}/tables/{table}/tableRoles/{role}"