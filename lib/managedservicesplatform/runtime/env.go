package runtime

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type requestedEnvVar struct {
	name         string
	defaultValue string
	description  string
}

type Env struct {
	errs []error
	env  map[string]string

	// requestedEnvVars are only available after ConfigLoader is used on this
	// Env instance.
	requestedEnvVars []requestedEnvVar
}

func newEnv() (*Env, error) {
	env := os.Environ()
	envMap := make(map[string]string, len(env))
	for _, e := range env {
		kv := strings.SplitN(e, "=", 2)
		if len(kv) != 2 {
			return nil, errors.Errorf("unable to parse environment variable %q", e)
		}
		envMap[kv[0]] = kv[1]
	}
	return &Env{
		errs: make([]error, 0),
		env:  envMap,
	}, nil
}

func (e *Env) get(name, defaultValue, description string) string {
	e.requestedEnvVars = append(e.requestedEnvVars, requestedEnvVar{
		name:         name,
		defaultValue: defaultValue,
		description:  description,
	})

	v, ok := e.env[name]
	if !ok {
		return defaultValue
	}
	return v
}

// validate returns any errors constructed from a Get* method after the values have
// been loaded from the environment.
func (e *Env) validate() error {
	if len(e.errs) == 0 {
		return nil
	}

	err := e.errs[0]
	for i := 1; i < len(e.errs); i++ {
		err = errors.Append(err, e.errs[i])
	}

	return err
}

// Get returns the value with the given name. If no value was supplied in the
// environment, the given default is used in its place. If no value is available,
// an error is added to the validation errors list.
func (e *Env) Get(name, defaultValue, description string) string {
	rawValue := e.get(name, defaultValue, description)
	if rawValue == "" {
		e.AddError(errors.Errorf("invalid value %q for %s: no value supplied, description: %s",
			rawValue, name, description))
		return ""
	}

	return rawValue
}

// GetOptional returns the value with the given name.
func (e *Env) GetOptional(name, description string) *string {
	v, ok := e.env[name]
	if !ok {
		return nil
	}
	return &v
}

// GetInt returns the value with the given name interpreted as an integer. If no
// value was supplied in the environment, the given default is used in its place.
// If no value is available, or if the given value or default cannot be converted
// to an integer, an error is added to the validation errors list.
func (e *Env) GetInt(name, defaultValue, description string) int {
	rawValue := e.get(name, defaultValue, description)
	i, err := strconv.ParseInt(rawValue, 10, 64)
	if err != nil {
		e.AddError(errors.Errorf("invalid int %q for %s: %s", rawValue, name, err))
		return 0
	}

	return int(i)
}

// GetPercent returns the value with the given name interpreted as an integer between
// 0 and 100. If no value was supplied in the environment, the given default is used
// in its place. If no value is available, if the given value or default cannot be
// converted to an integer, or if the value is out of the expected range, an error
// is added to the validation errors list.
func (e *Env) GetPercent(name, defaultValue, description string) int {
	value := e.GetInt(name, defaultValue, description)
	if value < 0 || value > 100 {
		e.AddError(errors.Errorf("invalid percent %q for %s: must be 0 <= p <= 100", value, name))
		return 0
	}

	return value
}

func (e *Env) GetInterval(name, defaultValue, description string) time.Duration {
	rawValue := e.get(name, defaultValue, description)
	d, err := time.ParseDuration(rawValue)
	if err != nil {
		e.AddError(errors.Errorf("invalid duration %q for %s: %s", rawValue, name, err))
		return 0
	}

	return d
}

// GetBool returns the value with the given name interpreted as a boolean. If no value was
// supplied in the environment, the given default is used in its place. If no value is available,
// or if the given value or default cannot be converted to a boolean, an error is added to the
// validation errors list.
func (e *Env) GetBool(name, defaultValue, description string) bool {
	rawValue := e.get(name, defaultValue, description)
	v, err := strconv.ParseBool(rawValue)
	if err != nil {
		e.AddError(errors.Errorf("invalid bool %q for %s: %s", rawValue, name, err))
		return false
	}

	return v
}

// AddError adds a validation error to the configuration object. This should be
// called from within the Load method of a decorated configuration object to have
// any effect.
func (e *Env) AddError(err error) {
	e.errs = append(e.errs, err)
}
