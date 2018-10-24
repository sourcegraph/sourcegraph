package conf

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/sourcegraph/jsonx"
	"github.com/sourcegraph/sourcegraph/pkg/jsonc"
	"github.com/sourcegraph/sourcegraph/schema"
)

// ParseConfigData reads the provided config string, but NOT the environment
func ParseConfigData(data string) (*schema.SiteConfiguration, error) {
	var tmpConfig schema.SiteConfiguration

	if data != "" {
		data, err := jsonc.Parse(data)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &tmpConfig); err != nil {
			return nil, err
		}
	}

	// For convenience, make sure this is not nil.
	if tmpConfig.ExperimentalFeatures == nil {
		tmpConfig.ExperimentalFeatures = &schema.ExperimentalFeatures{}
	}
	return &tmpConfig, nil
}

// parseConfig reads the provided string, then merges in additional
// data from the (deprecated) environment.
func parseConfig(data string) (*schema.SiteConfiguration, error) {
	tmpConfig, err := ParseConfigData(data)
	if err != nil {
		return nil, err
	}

	// Env var config takes highest precedence but is deprecated.
	if v, envVarNames, err := configFromEnv(); err != nil {
		return nil, err
	} else if len(envVarNames) > 0 {
		if err := json.Unmarshal(v, tmpConfig); err != nil {
			return nil, err
		}
	}
	return tmpConfig, nil
}

// TODO(slimsag): add back requireRestart and make use of it (it is a list of config properties that
// require restarting the given services to take effect)

// doNotRequireRestart is a list of options that do not require a service restart.
//
// TODO(slimsag): eliminate the need for this once all conf.GetTODO are removed.
var doNotRequireRestart = []string{
	"auth.allowSignup",
	"auth.public",
	"auth.userIdentityHTTPHeader",
	"github",
	"gitlab",
	"phabricator",
	"awsCodeCommit",
	"bitbucketServer",
	"repos.list",
	"gitMaxConcurrentClones",
	"repoListUpdateInterval",
	"gitolite",
	"gitOriginMap",
	"githubClientID",
	"githubClientSecret",
	"settings",
	"htmlHeadTop",
	"htmlHeadBottom",
	"htmlBodyTop",
	"htmlBodyBottom",
	"httpStrictTransportSecurity",
	"httpToHttpsRedirect",
	"disableBuiltInSearches",
	"email.smtp",
	"email.address",
	"disableAutoGitUpdates",
	"corsOrigin",
	"dontIncludeSymbolResultsByDefault",
	"langservers",
	"platform",
	"log",
	"experimentalFeatures::jumpToDefOSSIndex",
	"experimentalFeatures::canonicalURLRedirect",
	"experimentalFeatures::multipleAuthProviders",
	"experimentalFeatures::platform",
	"experimentalFeatures::discussions",
	"reviewBoard",
	"parentSourcegraph",
	"maxReposToSearch",
}

// merge a map, overwriting keys
func mergeMap(destMap, srcMap reflect.Value) {
	mapType := destMap.Type()
	if mapType.Kind() != reflect.Map {
		fmt.Printf("error: not a map: %T\n", destMap)
		return
	}
	valueType := mapType.Elem()
	zero := reflect.Zero(valueType)
	keys := srcMap.MapKeys()
	for _, key := range keys {
		srcValue := srcMap.MapIndex(key)
		destValue := destMap.MapIndex(key)
		switch srcValue.Kind() {
		case reflect.Struct:
			if destValue.IsNil() {
				destMap.SetMapIndex(key, srcValue)
			} else {
				mergeStruct(destValue.Interface(), srcValue.Interface())
			}
		case reflect.Slice:
			destMap.SetMapIndex(key, reflect.AppendSlice(destValue, srcValue))
		case reflect.Map:
			mergeMap(destValue, srcValue)
		default:
			if srcValue.Interface() != zero.Interface() {
				destMap.SetMapIndex(key, srcValue)
			}
		}
		destMap.SetMapIndex(key, srcMap.MapIndex(key))
	}
}

// merge a struct. recurse on structs, append arrays,
// overwrite everything else.
func mergeStruct(destInterface, srcInterface interface{}) {
	destType := reflect.TypeOf(destInterface)
	dest := reflect.ValueOf(destInterface)
	if destType.Kind() == reflect.Ptr {
		dest = dest.Elem()
		destType = dest.Type()
	}
	srcType := reflect.TypeOf(srcInterface)
	src := reflect.ValueOf(srcInterface)
	if srcType.Kind() == reflect.Ptr {
		src = src.Elem()
		srcType = src.Type()
	}
	if destType != srcType {
		fmt.Printf("fatal: destType '%T' and srcType '%T' are not equal.\n", dest, src)
		return
	}
	for i := 0; i < destType.NumField(); i++ {
		destField := dest.Field(i)
		srcField := src.Field(i)
		zero := reflect.Zero(destField.Type())
		switch destField.Kind() {
		case reflect.Struct:
			mergeStruct(destField, srcField)
		case reflect.Slice:
			destField.Set(reflect.AppendSlice(destField, srcField))
		case reflect.Map:
			mergeMap(destField, srcField)
		case reflect.Ptr:
			switch destField.Elem().Kind() {
			case reflect.Struct:
				srcValid := srcField.Elem().IsValid()
				destValid := destField.Elem().IsValid()
				if srcValid {
					if destValid {
						mergeStruct(destField.Interface(), srcField.Interface())
					} else {
						destField.Set(srcField)
					}
				}
			case reflect.Slice:
				destField.Elem().Set(reflect.AppendSlice(destField.Elem(), srcField.Elem()))
			case reflect.Map:
				mergeMap(destField.Elem(), srcField.Elem())
			}
		default:
			if srcField.Interface() != zero.Interface() {
				destField.Set(srcField)
			}
		}
	}
}

// recursively merge components of site config
func AppendConfig(dest, src *schema.SiteConfiguration) *schema.SiteConfiguration {
	if dest == nil {
		return src
	}
	if src == nil {
		return dest
	}
	mergeStruct(dest, src)
	return dest
}

// needRestartToApply determines if a restart is needed to apply the changes
// between the two configurations.
func needRestartToApply(before, after *schema.SiteConfiguration) bool {
	diff := diff(before, after)

	// Delete fields that do not require a process restart from the diff. Then
	// len(diff) > 0 tells us if we need to restart or not.
	for _, option := range doNotRequireRestart {
		delete(diff, option)
	}
	return len(diff) > 0
}

// diff returns names of the Go fields that have different values between the
// two configurations.
func diff(before, after *schema.SiteConfiguration) (fields map[string]struct{}) {
	fields = make(map[string]struct{})
	beforeFields := getJSONFields(before)
	afterFields := getJSONFields(after)
	for fieldName, beforeField := range beforeFields {
		afterField := afterFields[fieldName]
		if !reflect.DeepEqual(beforeField, afterField) {
			fields[fieldName] = struct{}{}
		}
	}
	return fields
}

func getJSONFields(vv interface{}) (fields map[string]interface{}) {
	fields = make(map[string]interface{})
	v := reflect.ValueOf(vv).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		tag := v.Type().Field(i).Tag.Get("json")
		if tag == "" {
			// should never happen, and if it does this func cannot work.
			panic(fmt.Sprintf("missing json struct field tag on %T field %q", v.Interface(), v.Type().Field(i).Name))
		}
		if ef, ok := f.Interface().(*schema.ExperimentalFeatures); ok && ef != nil {
			for fieldName, fieldValue := range getJSONFields(ef) {
				fields["experimentalFeatures::"+fieldName] = fieldValue
			}
			continue
		}
		fieldName := parseJSONTag(tag)
		fields[fieldName] = f.Interface()
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

// FormatOptions is the default format options that should be used for jsonx
// edit computation.
var FormatOptions = jsonx.FormatOptions{InsertSpaces: true, TabSize: 2, EOL: "\n"}
