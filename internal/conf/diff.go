package conf

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/schema"
)

func Diff(before, after string) (fields map[string]struct{}) {
	beforeCfg, err := ParseConfig(conftypes.RawUnified{Site: before})
	if err != nil {
		return nil
	}
	afterCfg, err := ParseConfig(conftypes.RawUnified{Site: after})
	if err != nil {
		return nil
	}
	return diff(beforeCfg, afterCfg)
}

// diff returns names of the Go fields that have different values between the
// two configurations.
func diff(before, after *Unified) (fields map[string]struct{}) {
	diff := diffStruct(before.SiteConfiguration, after.SiteConfiguration, "")
	for k, v := range diffStruct(before.ServiceConnectionConfig, after.ServiceConnectionConfig, "serviceConnections::") {
		diff[k] = v
	}
	return diff
}

func diffStruct(before, after any, prefix string) (fields map[string]struct{}) {
	fields = make(map[string]struct{})
	beforeFields := getJSONFields(before, prefix)
	afterFields := getJSONFields(after, prefix)
	for fieldName, beforeField := range beforeFields {
		afterField := afterFields[fieldName]
		if !reflect.DeepEqual(beforeField, afterField) {
			fields[fieldName] = struct{}{}
		}
	}
	return fields
}

func getJSONFields(vv any, prefix string) (fields map[string]any) {
	fields = make(map[string]any)
	v := reflect.ValueOf(vv)
	for i := range v.NumField() {
		f := v.Field(i)
		tag := v.Type().Field(i).Tag.Get("json")
		if tag == "" {
			// should never happen, and if it does this func cannot work.
			panic(fmt.Sprintf("missing json struct field tag on %T field %q", v.Interface(), v.Type().Field(i).Name))
		}
		if ef, ok := f.Interface().(*schema.ExperimentalFeatures); ok && ef != nil {
			for fieldName, fieldValue := range getJSONFields(*ef, prefix+"experimentalFeatures::") {
				fields[fieldName] = fieldValue
			}
			continue
		}
		fieldName := parseJSONTag(tag)
		fields[prefix+fieldName] = f.Interface()
	}
	return fields
}

// parseJSONTag parses a JSON struct field tag to return the JSON field name.
func parseJSONTag(tag string) string {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx]
	}
	return tag
}
