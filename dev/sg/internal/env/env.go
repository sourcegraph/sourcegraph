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

func New(name, value string, priority Priority) EnvVar {
	return EnvVar{
		Name:     name,
		Value:    value,
		Priority: priority,
	}
}

//func MergeEnvs(... map[string])
