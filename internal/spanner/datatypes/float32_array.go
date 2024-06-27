package datatypes

import (
	"database/sql/driver"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Float32Array is a custom type for a slice of float32 to be used with Spanner array type
type Float32Array []float32

// GormDataType gorm common data type
func (s *Float32Array) GormDataType() string {
	return "float32 array"
}

// GormDBDataType gorm db data type
func (s *Float32Array) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "spanner":
		return "ARRAY<FLOAT32>"
	}
	return ""
}

// Value implements the driver.Valuer interface for converting the Float32Array to a Spanner array
func (s *Float32Array) Value() (driver.Value, error) {
	return s, nil
}

// Scan implements the sql.Scanner interface for converting a Spanner array to a Float32Array
func (s *Float32Array) Scan(value interface{}) error {
	array, ok := value.([]float32)
	if !ok {
		return errors.New("type assertion to []float32 failed")
	}
	*s = Float32Array(array)
	return nil
}
