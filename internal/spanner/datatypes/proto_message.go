package datatypes

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type ProtoMessage struct {
	ProtoMessageVal proto.Message
	Valid           bool
}

func (m *ProtoMessage) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	if err := proto.Unmarshal(bytes, m.ProtoMessageVal); err != nil {
		return err
	}

	return nil
}

func (m ProtoMessage) Value() (driver.Value, error) {
	if m.ProtoMessageVal == nil {
		return nil, nil
	}

	bytes, err := proto.Marshal(m.ProtoMessageVal)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (m ProtoMessage) GormDataType() string {
	if m.ProtoMessageVal == nil {
		return ""
	}

	return fmt.Sprintf("%s", m.ProtoMessageVal.ProtoReflect().Descriptor().FullName())
}

func (m ProtoMessage) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	if m.ProtoMessageVal == nil {
		return ""
	}

	// use field.Tag, field.TagSettings gets field's tags
	// checkout https://github.com/go-gorm/gorm/blob/master/schema/field.go for all options
	return fmt.Sprintf("%s", m.ProtoMessageVal.ProtoReflect().Descriptor().FullName())
}

// IsNull implements NullableValue.IsNull for NullProtoMessage.
func (m ProtoMessage) IsNull() bool {
	return !m.Valid
}

// String implements Stringer.String for NullProtoMessage.
func (m ProtoMessage) String() string {
	if !m.Valid {
		return ""
	}
	return protoadapt.MessageV1Of(m.ProtoMessageVal).String()
}

// MarshalJSON implements json.Marshaler.MarshalJSON for NullProtoMessage.
func (m ProtoMessage) MarshalJSON() ([]byte, error) {
	if m.Valid {
		return proto.Marshal(m.ProtoMessageVal)
	}
	return nil, nil
}

// UnmarshalJSON implements json.Unmarshaler.UnmarshalJSON for NullProtoMessage.
func (m *ProtoMessage) UnmarshalJSON(payload []byte) error {
	if payload == nil {
		return fmt.Errorf("payload should not be nil")
	}
	if bytes.Equal(payload, []byte("")) {
		m.ProtoMessageVal = nil
		m.Valid = false
		return nil
	}
	if err := proto.Unmarshal(payload, m.ProtoMessageVal); err != nil {
		return fmt.Errorf("payload cannot be converted to a proto message: err: %s", err)
	}
	m.Valid = true
	return nil
}
