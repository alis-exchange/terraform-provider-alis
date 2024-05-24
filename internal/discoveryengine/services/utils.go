package services

import (
	"fmt"
)

func ValidateJsonSchema(sch map[string]interface{}, isTopLevel bool) error {
	allowedKeys := []string{
		"$schema",
		"type",
		"date_detection",
		"properties",
		"items",
		"keyPropertyMapping",
		"dimension",
		"retrievable",
		"dynamicFacetable",
		"searchable",
		"indexable",
		"completable",
	}

	// Validate allowed keys
	for key := range sch {
		keyFound := false
		for _, allowedKey := range allowedKeys {
			if key == allowedKey {
				keyFound = true
				break
			}
		}
		if !keyFound {
			return fmt.Errorf("key `%s` is not allowed", key)
		}
	}

	// Validate schema key
	schema, schemaOk := sch["$schema"]
	if schemaOk && !isTopLevel {
		return fmt.Errorf("`$schema` key is only allowed at the top level")
	}
	if !schemaOk && isTopLevel {
		return fmt.Errorf("missing `$schema` key")
	}
	if schemaOk {
		_, schemaCastOk := schema.(string)
		if !schemaCastOk {
			return fmt.Errorf("`$schema` must be a string")
		}
	}

	// Validate type key
	typeVal, typeOk := sch["type"]
	if !typeOk || typeVal == "" {
		return fmt.Errorf("missing `type` key")
	}
	_, typeCastOk := typeVal.(string)
	if !typeCastOk {
		return fmt.Errorf("`type` must be a string")
	}

	switch typeVal {
	case "number", "string", "boolean", "integer", "datetime", "geolocation":
		{
			// Validate allowed keys: keyPropertyMapping, type, dimension, retrievable, dynamicFacetable, searchable, indexable, completable
			if keyPropertyMapping, keyPropertyMappingOk := sch["keyPropertyMapping"]; keyPropertyMappingOk {
				_, keyPropertyMappingCastOk := keyPropertyMapping.(string)
				if !keyPropertyMappingCastOk {
					return fmt.Errorf("`keyPropertyMapping` must be a string")
				}
			}

			if dimension, dimensionOk := sch["dimension"]; dimensionOk {
				dimensionVal, dimensionCastOk := dimension.(float64)
				if !dimensionCastOk {
					return fmt.Errorf("`dimension` must be a valid number")
				}
				if dimensionVal < 1 || dimensionVal > 768 {
					return fmt.Errorf("`dimension` must be a valid number between 1 and 768")
				}
			}

			if retrievable, retrievableOk := sch["retrievable"]; retrievableOk {
				_, retrievableCastOk := retrievable.(bool)
				if !retrievableCastOk {
					return fmt.Errorf("`retrievable` must be a boolean")
				}
			}

			if dynamicFacetable, dynamicFacetableOk := sch["dynamicFacetable"]; dynamicFacetableOk {
				_, dynamicFacetableCastOk := dynamicFacetable.(bool)
				if !dynamicFacetableCastOk {
					return fmt.Errorf("`dynamicFacetable` must be a boolean")
				}
			}

			if searchable, searchableOk := sch["searchable"]; searchableOk {
				_, searchableCastOk := searchable.(bool)
				if !searchableCastOk {
					return fmt.Errorf("`searchable` must be a boolean")
				}
			}

			if indexable, indexableOk := sch["indexable"]; indexableOk {
				_, indexableCastOk := indexable.(bool)
				if !indexableCastOk {
					return fmt.Errorf("`indexable` must be a boolean")
				}
			}

			if completable, completableOk := sch["completable"]; completableOk {
				_, completableCastOk := completable.(bool)
				if !completableCastOk {
					return fmt.Errorf("`completable` must be a boolean")
				}
			}

		}
	case "object":
		{
			// Validate properties key
			properties, propertiesOk := sch["properties"]
			if !propertiesOk {
				return fmt.Errorf("missing `properties` key")
			}
			_, propertiesCastOk := properties.(map[string]interface{})
			if !propertiesCastOk {
				return fmt.Errorf("`properties` must be a map with string keys and map values")
			}

			// Iterate over properties and validate each one
			for key, value := range properties.(map[string]interface{}) {
				// Validate key
				if key == "" {
					return fmt.Errorf("property key cannot be empty")
				}

				// Validate value
				_, valueCastOk := value.(map[string]interface{})
				if !valueCastOk {
					return fmt.Errorf("value for key `%s` must be a map", key)
				}

				// Recursively validate value
				if err := ValidateJsonSchema(value.(map[string]interface{}), false); err != nil {
					return fmt.Errorf("error validating property `%s`: %v", key, err)
				}
			}
		}
	case "array":
		{
			// Validate items key
			items, itemsOk := sch["items"]
			if !itemsOk {
				return fmt.Errorf("missing `items` key")
			}
			_, itemsCastOk := items.(map[string]interface{})
			if !itemsCastOk {
				return fmt.Errorf("`items` must be a map")
			}

			// Recursively validate items
			if err := ValidateJsonSchema(items.(map[string]interface{}), false); err != nil {
				return fmt.Errorf("error validating `items`: %v", err)
			}
		}
	}

	return nil
}
