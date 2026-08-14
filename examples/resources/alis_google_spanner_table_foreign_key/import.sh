# Foreign key can be imported by specifying the fully qualified name of the constraint
# projects/{project}/instances/{instance}/databases/{database}/tables/{table}/constraints/{constraint}
terraform import alis_google_spanner_table_foreign_key.foreign_key "projects/{project}/instances/{instance}/databases/{database}/tables/{table}/constraints/{constraint}"