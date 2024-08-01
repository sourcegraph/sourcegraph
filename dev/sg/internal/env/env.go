package env

type Priority int

const (
	GlobalEnvPriority Priority = iota
	BaseCommandsetEnvPriority
	BaseCommandEnvPriority
	SecretEnvPriority
	OverrideCommandsetEnvPriority
	OverrideCommandEnvPriority
)

type EnvVar struct {
	Name  string
	Value string

	Priority Priority
}

func (e EnvVar) GetValue() string {
	return e.Value
}

func New(name, value string, priority Priority) EnvVar {
	return EnvVar{
		Name:     name,
		Value:    value,
		Priority: priority,
	}
}

func ConvertEnvMap(envvar map[string]string, priority Priority) map[string]EnvVar {
	envmap := make(map[string]EnvVar, len(envvar))
	for name, val := range envvar {
		envmap[name] = New(name, val, priority)
	}
	return envmap
}

//func ConsolidateEnvs(e map[string]EnvVar) map
