package config

import (
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"
)

func IsAllowedOverrideType(existing interface{}, v interface{}) bool {
	if v == nil {
		return false
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.Map:
		return false
		// only override with array if it has a length
	case reflect.Array, reflect.Slice:
		if reflect.ValueOf(v).Len() > 0 {
			return true
		} else {
			return false
		}
	case reflect.Int, reflect.Bool, reflect.String:
		// enable overriding with "", 0, false
		// warning: config objects should always use "omitempty" or _all_ fields will get overwritten
		return true
	}
	//nolint
	if reflect.ValueOf(v).IsZero() {
		// don't overwrite with 0 values or many things will get
		// overwritten
		return false
	}
	return true
}
func IsMap(v interface{}) bool {
	switch reflect.TypeOf(v).Kind() {
	case reflect.Map:
		return true
	}
	return false
}

func RecusiveOverride(defaults map[string]interface{}, overrides map[string]interface{}) {
	for key, val := range overrides {
		if existingVal, ok := defaults[key]; ok {
			if IsMap(existingVal) && IsMap(val) {
				switch existingVal.(type) {
				case map[string]interface{}:
					RecusiveOverride(existingVal.(map[string]interface{}), val.(map[string]interface{}))
				default:
					panic(fmt.Sprintf("unknown map: %T", existingVal))
				}
			} else {
				if IsAllowedOverrideType(existingVal, val) {
					defaults[key] = val
				} else {
					// should not overwrite full arrays or maps
				}
			}
		} else {
			// just add it
			defaults[key] = val
		}
	}

}

func ApplyDefaults(defaultCfg interface{}, overrideCfg interface{}, newCfg interface{}) error {
	bz, err := yaml.Marshal(defaultCfg)
	if err != nil {
		return err
	}
	defaults := map[string]interface{}{}
	err = yaml.Unmarshal(bz, &defaults)
	if err != nil {
		return err
	}

	bz, err = yaml.Marshal(overrideCfg)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{}
	err = yaml.Unmarshal(bz, &overrides)
	if err != nil {
		return err
	}
	RecusiveOverride(defaults, overrides)

	// serde to new cfg
	bz, err = yaml.Marshal(defaults)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(bz, newCfg)
}
