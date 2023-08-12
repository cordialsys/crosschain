package config

import (
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"
)

func IsAllowedOverrideType(v interface{}) bool {
	switch reflect.TypeOf(v).Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return false
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
				if IsAllowedOverrideType(val) {
					defaults[key] = val
					zero := reflect.ValueOf(val).IsZero()
					fmt.Println("override", key, val, "zero", zero)
				} else {
					// should not overwrite full arrays or maps
					fmt.Println("skip", key, val)
				}
			}
		} else {
			// just add it
			fmt.Println("added", key, val)
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
