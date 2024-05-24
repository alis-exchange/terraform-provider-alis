package services

import (
	"encoding/json"
	"testing"
)

func Test_validateJsonSchema(t *testing.T) {
	jsonString := "{\"properties\":{\"esgViewUpdate\":{\"properties\":{\"envScore\":{\"dimension\":100,\"dynamicFacetable\":true,\"type\":\"number\",\"indexable\":true,\"retrievable\":true},\"socialScore\":{\"indexable\":true,\"retrievable\":true,\"dynamicFacetable\":true,\"type\":\"number\"},\"govScore\":{\"type\":\"number\",\"dynamicFacetable\":true,\"retrievable\":true,\"indexable\":true},\"esgTotalScore\":{\"retrievable\":true,\"indexable\":true,\"dynamicFacetable\":true,\"type\":\"number\"}},\"type\":\"object\"},\"referencedByMemos\":{\"type\":\"array\",\"items\":{\"searchable\":true,\"dynamicFacetable\":true,\"indexable\":true,\"type\":\"string\",\"retrievable\":true}},\"state\":{\"type\":\"string\",\"dynamicFacetable\":true,\"indexable\":true,\"retrievable\":true,\"searchable\":true},\"updateTime\":{\"dynamicFacetable\":true,\"retrievable\":true,\"indexable\":true,\"type\":\"datetime\"},\"createTime\":{\"dynamicFacetable\":true,\"retrievable\":true,\"indexable\":true,\"type\":\"datetime\"},\"name\":{\"type\":\"string\",\"indexable\":true,\"searchable\":true,\"dynamicFacetable\":true,\"retrievable\":true},\"trade\":{\"type\":\"object\",\"properties\":{\"deal\":{\"properties\":{\"account\":{\"indexable\":true,\"retrievable\":true,\"dynamicFacetable\":true,\"type\":\"string\",\"searchable\":true},\"currentWeight\":{\"dynamicFacetable\":true,\"type\":\"number\",\"indexable\":true,\"retrievable\":true},\"reason\":{\"retrievable\":true,\"searchable\":true,\"dynamicFacetable\":true,\"type\":\"string\",\"indexable\":true},\"tradingGuidance\":{\"dynamicFacetable\":true,\"type\":\"string\",\"indexable\":true,\"searchable\":true,\"retrievable\":true},\"deal\":{\"retrievable\":true,\"type\":\"string\",\"indexable\":true,\"dynamicFacetable\":true,\"searchable\":true},\"targetWeight\":{\"retrievable\":true,\"dynamicFacetable\":true,\"type\":\"number\",\"indexable\":true}},\"type\":\"object\"}}},\"recommendation\":{\"type\":\"object\",\"properties\":{\"tradeRecommendation\":{\"dynamicFacetable\":true,\"type\":\"string\",\"indexable\":true,\"searchable\":true,\"retrievable\":true},\"tradeRecommendationConviction\":{\"indexable\":true,\"searchable\":true,\"type\":\"string\",\"retrievable\":true,\"dynamicFacetable\":true},\"levelOfUnderstanding\":{\"dynamicFacetable\":true,\"type\":\"string\",\"indexable\":true,\"retrievable\":true,\"searchable\":true}}},\"content\":{\"properties\":{\"pdf\":{\"type\":\"object\",\"properties\":{\"uri\":{\"retrievable\":true,\"dynamicFacetable\":true,\"searchable\":true,\"indexable\":true,\"type\":\"string\"}}},\"email\":{\"properties\":{\"htmlContentUri\":{\"indexable\":true,\"searchable\":true,\"type\":\"string\",\"dynamicFacetable\":true,\"retrievable\":true},\"sender\":{\"retrievable\":true,\"type\":\"string\",\"dynamicFacetable\":true,\"searchable\":true,\"indexable\":true},\"subject\":{\"retrievable\":true,\"type\":\"string\",\"searchable\":true,\"indexable\":true,\"dynamicFacetable\":true}},\"type\":\"object\"}},\"type\":\"object\"},\"referencedMemos\":{\"type\":\"array\",\"items\":{\"retrievable\":true,\"dynamicFacetable\":true,\"searchable\":true,\"type\":\"string\",\"indexable\":true}},\"longSummary\":{\"type\":\"string\",\"dynamicFacetable\":true,\"indexable\":true,\"searchable\":true,\"retrievable\":true},\"estimateRevision\":{\"type\":\"object\",\"properties\":{\"revision\":{\"retrievable\":true,\"indexable\":true,\"type\":\"string\",\"searchable\":true,\"dynamicFacetable\":true},\"foreignEarnings\":{\"indexable\":true,\"dynamicFacetable\":true,\"type\":\"number\",\"retrievable\":true},\"sustainableGrowthRates\":{\"properties\":{\"twoYear\":{\"retrievable\":true,\"dynamicFacetable\":true,\"indexable\":true,\"type\":\"number\"},\"sevenYear\":{\"type\":\"number\",\"dynamicFacetable\":true,\"retrievable\":true,\"indexable\":true}},\"type\":\"object\"}}},\"references\":{\"items\":{\"properties\":{\"resourceType\":{\"dynamicFacetable\":true,\"indexable\":true,\"type\":\"string\",\"retrievable\":true,\"searchable\":true},\"name\":{\"indexable\":true,\"type\":\"string\",\"retrievable\":true,\"dynamicFacetable\":true,\"searchable\":true}},\"type\":\"object\"},\"type\":\"array\"},\"creator\":{\"dynamicFacetable\":true,\"retrievable\":true,\"indexable\":true,\"type\":\"string\",\"searchable\":true},\"memoType\":{\"searchable\":true,\"retrievable\":true,\"type\":\"string\",\"indexable\":true,\"dynamicFacetable\":true},\"title\":{\"keyPropertyMapping\":\"title\",\"retrievable\":true,\"type\":\"string\"},\"environmentViewUpdate\":{\"type\":\"object\",\"properties\":{\"external\":{\"type\":\"string\",\"searchable\":true,\"dynamicFacetable\":true,\"retrievable\":true,\"indexable\":true},\"internal\":{\"dynamicFacetable\":true,\"searchable\":true,\"type\":\"string\",\"indexable\":true,\"retrievable\":true}}},\"shortSummary\":{\"indexable\":true,\"dynamicFacetable\":true,\"retrievable\":true,\"type\":\"string\",\"searchable\":true}},\"type\":\"object\",\"$schema\":\"https://json-schema.org/draft/2020-12/schema\"}"

	var jsonMap map[string]interface{}
	err := json.Unmarshal([]byte(jsonString), &jsonMap)
	if err != nil {
		t.Errorf("Error unmarshalling JSON: %v", err)
		return
	}

	type args struct {
		sch        map[string]interface{}
		isTopLevel bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test ValidateJsonSchema",
			args: args{
				sch:        jsonMap,
				isTopLevel: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateJsonSchema(tt.args.sch, tt.args.isTopLevel); (err != nil) != tt.wantErr {
				t.Errorf("ValidateJsonSchema() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
