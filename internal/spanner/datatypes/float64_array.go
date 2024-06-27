package datatypes

import (
	"database/sql/driver"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Float64Array is a custom type for a slice of float64 to be used with Spanner array type
type Float64Array []float64

// GormDataType gorm common data type
func (s *Float64Array) GormDataType() string {
	return "float64 array"
}

// GormDBDataType gorm db data type
func (s *Float64Array) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "spanner":
		return "ARRAY<FLOAT64>"
	}
	return ""
}

// Value implements the driver.Valuer interface for converting the Float64Array to a Spanner array
func (s *Float64Array) Value() (driver.Value, error) {
	return s, nil
}

// Scan implements the sql.Scanner interface for converting a Spanner array to a Float64Array
func (s *Float64Array) Scan(value interface{}) error {
	array, ok := value.([]float64)
	if !ok {
		return errors.New("type assertion to []float64 failed")
	}
	*s = Float64Array(array)
	return nil
}
