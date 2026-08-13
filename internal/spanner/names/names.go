// Package names owns the resource-name grammar used across the provider:
// projects/{p}/instances/{i}/databases/{d}[/{collection}/{id}]. Parsing
// returns typed errors on short or malformed input — never panics, never
// silent misparses — and formatting reproduces the historical layouts
// byte-identically.
package names

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidName is wrapped by every parse failure, so callers can branch
// with errors.Is without matching message text.
var ErrInvalidName = errors.New("invalid resource name")

// parseSegments validates name against alternating collection/id segments
// (e.g. "projects", "instances", "databases") and returns the ids in order.
func parseSegments(name string, collections ...string) ([]string, error) {
	parts := strings.Split(name, "/")
	if len(parts) != 2*len(collections) {
		return nil, fmt.Errorf("%w: %q must have the form %s", ErrInvalidName, name, format(collections))
	}
	ids := make([]string, 0, len(collections))
	for i, collection := range collections {
		if parts[2*i] != collection {
			return nil, fmt.Errorf("%w: %q must have the form %s", ErrInvalidName, name, format(collections))
		}
		if parts[2*i+1] == "" {
			return nil, fmt.Errorf("%w: %q has an empty %s id", ErrInvalidName, name, collection)
		}
		ids = append(ids, parts[2*i+1])
	}
	return ids, nil
}

// format renders the expected shape for error messages, e.g.
// "projects/{project}/instances/{instance}".
func format(collections []string) string {
	segments := make([]string, 0, len(collections))
	for _, c := range collections {
		segments = append(segments, c+"/{"+strings.TrimSuffix(c, "s")+"}")
	}
	return strings.Join(segments, "/")
}

// DatabaseName is projects/{p}/instances/{i}/databases/{d}.
type DatabaseName struct {
	Project  string
	Instance string
	Database string
}

// ParseDatabase parses a DatabaseName; failures wrap ErrInvalidName.
func ParseDatabase(name string) (DatabaseName, error) {
	ids, err := parseSegments(name, "projects", "instances", "databases")
	if err != nil {
		return DatabaseName{}, err
	}
	return DatabaseName{Project: ids[0], Instance: ids[1], Database: ids[2]}, nil
}

func (n DatabaseName) String() string {
	return fmt.Sprintf("projects/%s/instances/%s/databases/%s", n.Project, n.Instance, n.Database)
}

// TableName is projects/{p}/instances/{i}/databases/{d}/tables/{t}.
type TableName struct {
	Project  string
	Instance string
	Database string
	Table    string
}

// ParseTable parses a TableName; failures wrap ErrInvalidName.
func ParseTable(name string) (TableName, error) {
	ids, err := parseSegments(name, "projects", "instances", "databases", "tables")
	if err != nil {
		return TableName{}, err
	}
	return TableName{Project: ids[0], Instance: ids[1], Database: ids[2], Table: ids[3]}, nil
}

func (n TableName) String() string {
	return fmt.Sprintf("%s/tables/%s", n.DatabaseName().String(), n.Table)
}

// DatabaseName returns the parent database's name.
func (n TableName) DatabaseName() DatabaseName {
	return DatabaseName{Project: n.Project, Instance: n.Instance, Database: n.Database}
}

// SequenceName is projects/{p}/instances/{i}/databases/{d}/sequences/{s}.
type SequenceName struct {
	Project  string
	Instance string
	Database string
	Sequence string
}

// ParseSequence parses a SequenceName; failures wrap ErrInvalidName.
func ParseSequence(name string) (SequenceName, error) {
	ids, err := parseSegments(name, "projects", "instances", "databases", "sequences")
	if err != nil {
		return SequenceName{}, err
	}
	return SequenceName{Project: ids[0], Instance: ids[1], Database: ids[2], Sequence: ids[3]}, nil
}

func (n SequenceName) String() string {
	return fmt.Sprintf("%s/sequences/%s", n.DatabaseName().String(), n.Sequence)
}

// DatabaseName returns the parent database's name.
func (n SequenceName) DatabaseName() DatabaseName {
	return DatabaseName{Project: n.Project, Instance: n.Instance, Database: n.Database}
}

// DatabaseRoleName is projects/{p}/instances/{i}/databases/{d}/databaseRoles/{r}.
type DatabaseRoleName struct {
	Project  string
	Instance string
	Database string
	Role     string
}

// ParseDatabaseRole parses a DatabaseRoleName; failures wrap ErrInvalidName.
func ParseDatabaseRole(name string) (DatabaseRoleName, error) {
	ids, err := parseSegments(name, "projects", "instances", "databases", "databaseRoles")
	if err != nil {
		return DatabaseRoleName{}, err
	}
	return DatabaseRoleName{Project: ids[0], Instance: ids[1], Database: ids[2], Role: ids[3]}, nil
}

func (n DatabaseRoleName) String() string {
	return fmt.Sprintf("%s/databaseRoles/%s", n.DatabaseName().String(), n.Role)
}

// DatabaseName returns the parent database's name.
func (n DatabaseRoleName) DatabaseName() DatabaseName {
	return DatabaseName{Project: n.Project, Instance: n.Instance, Database: n.Database}
}

// IndexName is projects/{p}/instances/{i}/databases/{d}/tables/{t}/indexes/{x}.
type IndexName struct {
	Project  string
	Instance string
	Database string
	Table    string
	Index    string
}

// ParseIndex parses an IndexName; failures wrap ErrInvalidName.
func ParseIndex(name string) (IndexName, error) {
	ids, err := parseSegments(name, "projects", "instances", "databases", "tables", "indexes")
	if err != nil {
		return IndexName{}, err
	}
	return IndexName{Project: ids[0], Instance: ids[1], Database: ids[2], Table: ids[3], Index: ids[4]}, nil
}

func (n IndexName) String() string {
	return fmt.Sprintf("%s/indexes/%s", n.TableName().String(), n.Index)
}

// TableName returns the parent table's name.
func (n IndexName) TableName() TableName {
	return TableName{Project: n.Project, Instance: n.Instance, Database: n.Database, Table: n.Table}
}

// TableRoleName is projects/{p}/instances/{i}/databases/{d}/tables/{t}/tableRoles/{r}
// — the import-ID shape of a table IAM binding.
type TableRoleName struct {
	Project  string
	Instance string
	Database string
	Table    string
	Role     string
}

// ParseTableRole parses a TableRoleName; failures wrap ErrInvalidName.
func ParseTableRole(name string) (TableRoleName, error) {
	ids, err := parseSegments(name, "projects", "instances", "databases", "tables", "tableRoles")
	if err != nil {
		return TableRoleName{}, err
	}
	return TableRoleName{Project: ids[0], Instance: ids[1], Database: ids[2], Table: ids[3], Role: ids[4]}, nil
}

func (n TableRoleName) String() string {
	return fmt.Sprintf("%s/tableRoles/%s", n.TableName().String(), n.Role)
}

// TableName returns the parent table's name.
func (n TableRoleName) TableName() TableName {
	return TableName{Project: n.Project, Instance: n.Instance, Database: n.Database, Table: n.Table}
}

// ForeignKeyName is projects/{p}/instances/{i}/databases/{d}/tables/{t}/constraints/{c}
// — the import-ID shape of a table foreign-key constraint.
type ForeignKeyName struct {
	Project    string
	Instance   string
	Database   string
	Table      string
	Constraint string
}

// ParseForeignKey parses a ForeignKeyName; failures wrap ErrInvalidName.
func ParseForeignKey(name string) (ForeignKeyName, error) {
	ids, err := parseSegments(name, "projects", "instances", "databases", "tables", "constraints")
	if err != nil {
		return ForeignKeyName{}, err
	}
	return ForeignKeyName{Project: ids[0], Instance: ids[1], Database: ids[2], Table: ids[3], Constraint: ids[4]}, nil
}

func (n ForeignKeyName) String() string {
	return fmt.Sprintf("%s/constraints/%s", n.TableName().String(), n.Constraint)
}

// TableName returns the parent table's name.
func (n ForeignKeyName) TableName() TableName {
	return TableName{Project: n.Project, Instance: n.Instance, Database: n.Database, Table: n.Table}
}
