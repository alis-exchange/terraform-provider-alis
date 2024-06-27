package datatypes

import (
	"database/sql/driver"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Int64Array is a custom type for a slice of int64 to be used with Spanner array type
type Int64Array []int64

// GormDataType gorm common data type
func (s *Int64Array) GormDataType() string {
	return "int64 array"
}

// GormDBDataType gorm db data type
func (s *Int64Array) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "spanner":
		return "ARRAY<INT64>"
	}
	return ""
}

// Value implements the driver.Valuer interface for converting the Int64Array to a Spanner array
func (s *Int64Array) Value() (driver.Value, error) {
	return s, nil
}

// Scan implements the sql.Scanner interface for converting a Spanner array to a Int64Array
func (s *Int64Array) Scan(value interface{}) error {
	array, ok := value.([]int64)
	if !ok {
		return errors.New("type assertion to []int64 failed")
	}
	*s = Int64Array(array)
	return nil
}
