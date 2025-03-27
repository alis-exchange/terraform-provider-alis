package schema

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"terraform-provider-alis/internal/utils"
)

type ColumnMetadataMeta struct {
	Type                        string `json:"type"`
	Size                        string `json:"size"`
	Precision                   string `json:"precision"`
	Scale                       string `json:"scale"`
	Required                    string `json:"required"`
	AutoIncrement               string `json:"auto_increment"`
	Unique                      string `json:"unique"`
	AutoCreateTime              string `json:"auto_create_time"`
	AutoUpdateTime              string `json:"auto_update_time"`
	DefaultValue                string `json:"default_value"`
	IsPrimaryKey                string `json:"is_primary_key"`
	IsComputed                  string `json:"is_computed"`
	ComputationDdl              string `json:"computation_ddl"`
	ProtoPackage                string `json:"proto_package"`
	FileDescriptorSetPath       string `json:"file_descriptor_set_path"`
	FileDescriptorSetPathSource string `json:"file_descriptor_set_path_source"`
}

// Scan scans value into Jsonb and implements sql.Scanner interface
func (c *ColumnMetadataMeta) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &c)
}

// Value returns value of CustomerInfo struct and implements driver.Valuer interface
func (c ColumnMetadataMeta) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c ColumnMetadataMeta) GormDataType() string {
	return "bytes"
}

type ColumnMetadata struct {
	TableName  string              `gorm:"primaryKey"`
	ColumnName string              `gorm:"primaryKey"`
	Metadata   *ColumnMetadataMeta `gorm:"type:bytes"`
	CreatedAt  time.Time           // Automatically managed by GORM for creation time
	UpdatedAt  time.Time           // Automatically managed by GORM for update time
}

func GetColumnMetadata(db *gorm.DB, tableName string) ([]*ColumnMetadata, error) {
	// Create or Update ColumnMetadata table
	if err := db.AutoMigrate(&ColumnMetadata{}); err != nil {
		return nil, err
	}

	// Get ColumnMetadata table
	var results []*ColumnMetadata
	if err := db.Where("table_name = ?", tableName).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
func UpdateColumnMetadata(ctx context.Context, db *gorm.DB, tableName string, columns []*SpannerTableColumn) error {
	// Create or Update ColumnMetadata table
	// IMPORTANT: When tables don't depend on each other, terraform will attempt to create them in parallel.
	// This can cause the migration to run at the same time for multiple tables, which can lead to a duplicate table error.
	// To prevent this, we'll retry the migration a few times.
	_, err := utils.Retry(5, 10*time.Second, func() (interface{}, error) {
		if err := db.AutoMigrate(&ColumnMetadata{}); err != nil {
			return nil, err
		}

		return nil, nil
	})
	if err != nil {
		return err
	}

	// Update ColumnMetadata table
	for _, column := range columns {
		meta := &ColumnMetadataMeta{}
		meta.Type = column.Type
		if column.Size != nil {
			meta.Size = fmt.Sprintf("%d", column.Size.GetValue())
		} else {
			meta.Size = "nil"
		}
		if column.Required != nil {
			meta.Required = fmt.Sprintf("%t", column.Required.GetValue())
		} else {
			meta.Required = "nil"
		}
		if column.AutoCreateTime != nil {
			meta.AutoCreateTime = fmt.Sprintf("%t", column.AutoCreateTime.GetValue())
		} else {
			meta.AutoCreateTime = "nil"
		}
		if column.AutoUpdateTime != nil {
			meta.AutoUpdateTime = fmt.Sprintf("%t", column.AutoUpdateTime.GetValue())
		} else {
			meta.AutoUpdateTime = "nil"
		}
		if column.DefaultValue != nil {
			meta.DefaultValue = column.DefaultValue.GetValue()
		} else {
			meta.DefaultValue = "nil"
		}
		if column.IsPrimaryKey != nil {
			meta.IsPrimaryKey = fmt.Sprintf("%t", column.IsPrimaryKey.GetValue())
		} else {
			meta.IsPrimaryKey = "nil"
		}
		if column.IsComputed != nil {
			meta.IsComputed = fmt.Sprintf("%t", column.IsComputed.GetValue())
		} else {
			meta.IsComputed = "nil"
		}
		if column.ComputationDdl != nil {
			meta.ComputationDdl = column.ComputationDdl.GetValue()
		} else {
			meta.ComputationDdl = "nil"
		}
		if column.ProtoFileDescriptorSet != nil {
			if column.ProtoFileDescriptorSet.ProtoPackage != nil {
				meta.ProtoPackage = column.ProtoFileDescriptorSet.ProtoPackage.GetValue()
			} else {
				meta.ProtoPackage = "nil"
			}
			if column.ProtoFileDescriptorSet.FileDescriptorSetPath != nil {
				meta.FileDescriptorSetPath = column.ProtoFileDescriptorSet.FileDescriptorSetPath.GetValue()
			} else {
				meta.FileDescriptorSetPath = "nil"
			}
			if column.ProtoFileDescriptorSet.FileDescriptorSetPathSource != ProtoFileDescriptorSetSourceUnspecified {
				meta.FileDescriptorSetPathSource = fmt.Sprintf("%d", column.ProtoFileDescriptorSet.FileDescriptorSetPathSource)
			} else {
				meta.FileDescriptorSetPathSource = "nil"
			}
		}

		createRes := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&ColumnMetadata{
			TableName:  tableName,
			ColumnName: column.Name,
			Metadata:   meta,
		})
		if createRes.Error != nil {
			if status.Code(createRes.Error) != codes.AlreadyExists {
				return createRes.Error
			}

			updateRes := db.Clauses(clause.OnConflict{UpdateAll: true}).Save(&ColumnMetadata{
				TableName:  tableName,
				ColumnName: column.Name,
				Metadata:   meta,
			})
			if updateRes.Error != nil {
				return updateRes.Error
			}
		}
	}

	return nil
}
func DeleteColumnMetadata(db *gorm.DB, tableName string, columns []*SpannerTableColumn) error {
	// Create or Update ColumnMetadata table
	if err := db.AutoMigrate(&ColumnMetadata{}); err != nil {
		return err
	}

	// Delete ColumnMetadata table
	if columns == nil || len(columns) == 0 {
		deleteRes := db.Where("table_name = ?", tableName).Delete(&ColumnMetadata{})
		if deleteRes.Error != nil {
			return deleteRes.Error
		}
	} else {
		for _, column := range columns {
			deleteRes := db.Where("table_name = ? AND column_name = ?", tableName, column.Name).Delete(&ColumnMetadata{})
			if deleteRes.Error != nil {
				return deleteRes.Error
			}
		}
	}

	return nil
}
