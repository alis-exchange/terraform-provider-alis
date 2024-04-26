package services

import (
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
)

const (
	GCPolicyModeIntersection = "INTERSECTION"
	GCPolicyModeUnion        = "UNION"
)

// GetGcPolicyFromJSON recursively converts a JSON map to a bigtable.GCPolicy.
func GetGcPolicyFromJSON(inputPolicy map[string]interface{}, isTopLevel bool) (bigtable.GCPolicy, error) {
	var policies []bigtable.GCPolicy

	if err := validateNestedPolicy(inputPolicy, isTopLevel); err != nil {
		return nil, err
	}

	for _, p := range inputPolicy["rules"].([]interface{}) {
		childPolicy := p.(map[string]interface{})
		if err := validateNestedPolicy(childPolicy /*isTopLevel=*/, false); err != nil {
			return nil, err
		}

		if childPolicy["max_age"] != nil {
			maxAge := childPolicy["max_age"].(string)
			duration, err := time.ParseDuration(maxAge)
			if err != nil {
				return nil, fmt.Errorf("invalid duration string: %v", maxAge)
			}

			policies = append(policies, bigtable.MaxAgePolicy(duration))
		}

		if childPolicy["max_version"] != nil {
			version := childPolicy["max_version"].(float64)

			policies = append(policies, bigtable.MaxVersionsPolicy(int(version)))
		}

		if childPolicy["mode"] != nil {
			p, err := GetGcPolicyFromJSON(childPolicy, false)
			if err != nil {
				return nil, err
			}
			policies = append(policies, p)
		}
	}

	if inputPolicy["mode"] == nil {
		return policies[0], nil
	}
	switch strings.ToLower(inputPolicy["mode"].(string)) {
	case strings.ToLower(GCPolicyModeUnion):
		return bigtable.UnionPolicy(policies...), nil
	case strings.ToLower(GCPolicyModeIntersection):
		return bigtable.IntersectionPolicy(policies...), nil
	default:
		return policies[0], nil
	}
}

// GcPolicyToGcRuleMap Recursively converts Bigtable GC policy to JSON format in a map.
func GcPolicyToGcRuleMap(gcPolicy bigtable.GCPolicy, topLevel bool) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	switch bigtable.GetPolicyType(gcPolicy) {
	case bigtable.PolicyMaxAge:
		// Assert the type to get time.Duration
		age := time.Duration(gcPolicy.(bigtable.MaxAgeGCPolicy)).String()
		if topLevel {
			rule := make(map[string]interface{})
			rule["max_age"] = age
			rules := []interface{}{}
			rules = append(rules, rule)
			result["rules"] = rules
		} else {
			result["max_age"] = age
		}
		break
	case bigtable.PolicyMaxVersion:
		// Assert the type to get float64
		version := float64(gcPolicy.(bigtable.MaxVersionsGCPolicy))
		if topLevel {
			rule := make(map[string]interface{})
			rule["max_version"] = version
			rules := []interface{}{}
			rules = append(rules, rule)
			result["rules"] = rules
		} else {
			result["max_version"] = version
		}
		break
	case bigtable.PolicyUnion:
		// Assert the type to get []bigtable.GCPolicy
		policies := gcPolicy.(bigtable.UnionGCPolicy).Children

		result["mode"] = "union"
		rules := []interface{}{}
		for _, p := range policies {
			gcRuleMap, err := GcPolicyToGcRuleMap(p, false)
			if err != nil {
				return nil, err
			}
			rules = append(rules, gcRuleMap)
		}
		result["rules"] = rules
		break
	case bigtable.PolicyIntersection:
		// Assert the type to get []bigtable.GCPolicy
		policies := gcPolicy.(bigtable.IntersectionGCPolicy).Children

		result["mode"] = "intersection"
		rules := []interface{}{}
		for _, p := range policies {
			gcRuleMap, err := GcPolicyToGcRuleMap(p, false)
			if err != nil {
				return nil, err
			}
			rules = append(rules, gcRuleMap)
		}
		result["rules"] = rules
		break
	default:
		break
	}

	if err := validateNestedPolicy(result, topLevel); err != nil {
		return nil, err
	}

	return result, nil
}

func validateNestedPolicy(p map[string]interface{}, isTopLevel bool) error {
	if len(p) > 2 {
		return fmt.Errorf("rules has more than 2 fields")
	}
	maxVersion, maxVersionOk := p["max_version"]
	maxAge, maxAgeOk := p["max_age"]
	rulesObj, rulesOk := p["rules"]

	mode, modeOk := p["mode"]
	rules, arrOk := rulesObj.([]interface{})
	_, vCastOk := maxVersion.(float64)
	_, aCastOk := maxAge.(string)

	if rulesOk && !arrOk {
		return fmt.Errorf("`rules` must be array")
	}

	if modeOk && (strings.ToLower(mode.(string)) != strings.ToLower(GCPolicyModeUnion) && strings.ToLower(mode.(string)) != strings.ToLower(GCPolicyModeIntersection)) {
		return fmt.Errorf("`mode` must be either `union` or `intersection`")
	}

	if modeOk && len(rules) < 2 {
		return fmt.Errorf("`rules` need at least 2 GC rule when mode is specified")
	}

	if isTopLevel && !rulesOk {
		return fmt.Errorf("invalid nested policy, need `rules`")
	}

	if isTopLevel && !modeOk && len(rules) != 1 {
		return fmt.Errorf("when `mode` is not specified, `rules` can only have 1 child rule")
	}

	if !isTopLevel && len(p) == 2 && (!modeOk || !rulesOk) {
		return fmt.Errorf("need `mode` and `rules` for child nested policies")
	}

	if !isTopLevel && len(p) == 1 && !maxVersionOk && !maxAgeOk {
		return fmt.Errorf("need `max_version` or `max_age` for the rule")
	}

	if maxVersionOk && !vCastOk {
		return fmt.Errorf("`max_version` must be a number")
	}

	if maxAgeOk && !aCastOk {
		return fmt.Errorf("`max_age must be a string")
	}

	return nil
}
