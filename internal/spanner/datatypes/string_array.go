package datatypes

import (
	"database/sql/driver"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// StringArray is a custom type for a slice of strings to be used with Spanner array type
type StringArray []string

// GormDataType gorm common data type
func (s *StringArray) GormDataType() string {
	return "string array"
}

// GormDBDataType gorm db data type
func (s *StringArray) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "spanner":
		return "ARRAY<STRING(MAX)>"
	}
	return ""
}

// Value implements the driver.Valuer interface for converting the StringArray to a Spanner array
func (s *StringArray) Value() (driver.Value, error) {
	return s, nil
}

// Scan implements the sql.Scanner interface for converting a Spanner array to a StringArray
func (s *StringArray) Scan(value interface{}) error {
	array, ok := value.([]string)
	if !ok {
		return errors.New("type assertion to []string failed")
	}
	*s = StringArray(array)
	return nil
}
