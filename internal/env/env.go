package env

import (
	"expvar"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
)

type envflag struct {
	name        string
	description string
	value       string
}

var env []envflag
var environ map[string]string
var locked = false

var (
	// MyName represents the name of the current process.
	MyName, envVarName = findName()
	LogLevel           = Get("SRC_LOG_LEVEL", "warn", "upper log level to restrict log output to (dbug, info, warn, error, crit)")
	LogFormat          = Get("SRC_LOG_FORMAT", "logfmt", "log format (logfmt, condensed, json)")
	InsecureDev, _     = strconv.ParseBool(Get("INSECURE_DEV", "false", "Running in insecure dev (local laptop) mode"))
)

var (
	// DebugOut is os.Stderr if LogLevel includes dbug
	DebugOut io.Writer
	// InfoOut is os.Stderr if LogLevel includes info
	InfoOut io.Writer
	// WarnOut is os.Stderr if LogLevel includes warn
	WarnOut io.Writer
	// ErrorOut is os.Stderr if LogLevel includes error
	ErrorOut io.Writer
	// CritOut is os.Stderr if LogLevel includes crit
	CritOut io.Writer
)

// findName returns the name of the current process, that being the
// part of argv[0] after the last slash if any, and also the lowercase
// letters from that, suitable for use as a likely key for lookups
// in things like shell environment variables which can't contain
// hyphens.
func findName() (string, string) {
	// Environment variable names can't contain, for instance, hyphens.
	origName := filepath.Base(os.Args[0])
	name := strings.Replace(origName, "-", "_", -1)
	if name == "" {
		name = "unknown"
	}
	return origName, name
}

// Ensure behaves like Get except that it sets the environment variable if it doesn't exist.
func Ensure(name, defaultValue, description string) string {
	value := Get(name, defaultValue, description)
	if value == defaultValue {
		err := os.Setenv(name, value)
		if err != nil {
			panic(fmt.Sprintf("failed to set %s environment variable: %v", name, err))
		}
	}

	return value
}

func init() {
	lvl, _ := log15.LvlFromString(LogLevel)
	lvlFilterStderr := func(maxLvl log15.Lvl) io.Writer {
		// Note that log15 values look like e.g. LvlCrit == 0, LvlDebug == 4
		if lvl > maxLvl {
			return ioutil.Discard
		}
		return os.Stderr
	}
	DebugOut = lvlFilterStderr(log15.LvlDebug)
	InfoOut = lvlFilterStderr(log15.LvlInfo)
	WarnOut = lvlFilterStderr(log15.LvlWarn)
	ErrorOut = lvlFilterStderr(log15.LvlError)
	CritOut = lvlFilterStderr(log15.LvlCrit)
}

// Get returns the value of the given environment variable. It also registers the description for
// PrintHelp. Calling Get with the same name twice causes a panic. Get should only be called on
// package initialization. Calls at a later point will cause a panic if Lock was called before.
//
// This should be used for only *internal* environment values. User-visible configuration should be
// added to the Config struct in the github.com/sourcegraph/sourcegraph/config package.
func Get(name, defaultValue, description string) string {
	if locked {
		panic("env.Get has to be called on package initialization")
	}

	// os.LookupEnv is a syscall. We use Get a lot on startup in many
	// packages. This leads to it being the main contributor to init being
	// slow. So we avoid the constant syscalls by checking env once.
	if environ == nil {
		environ = environMap(os.Environ())
	}

	// Allow per-process override. For instance, SRC_LOG_LEVEL_repo_updater would
	// apply to repo-updater, but not to anything else.
	perProg := name + "_" + envVarName
	value, ok := environ[perProg]
	if !ok {
		value, ok = environ[name]
		if !ok {
			value = defaultValue
		}
	}

	env = append(env, envflag{
		name:        name,
		description: description,
		value:       value,
	})

	return value
}

// MustGetDuration is similar to Get but ensures that the value is a valid time.Duration.
func MustGetDuration(name string, defaultValue time.Duration, description string) time.Duration {
	s := Get(name, defaultValue.String(), description)
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(fmt.Sprintf("parsing environment variable %q. Expected valid time.Duration, got %q", name, s))
	}
	return d
}

// MustGetInt is similar to Get but ensures that the value is a valid int.
func MustGetInt(name string, defaultValue int, description string) int {
	s := Get(name, strconv.Itoa(defaultValue), description)
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(fmt.Sprintf("parsing environment variable %q. Expected valid integer, got %q", name, s))
	}
	return i
}

func environMap(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, e := range environ {
		i := strings.Index(e, "=")
		m[e[:i]] = e[i+1:]
	}
	return m
}

// Lock makes later calls to Get fail with a panic. Call this at the beginning of the main function.
func Lock() {
	locked = true

	sort.Slice(env, func(i, j int) bool { return env[i].name < env[j].name })

	for i := 1; i < len(env); i++ {
		if env[i-1].name == env[i].name {
			panic(fmt.Sprintf("%q already registered", env[i].name))
		}
	}

	expvar.Publish("env", expvar.Func(func() interface{} {
		return env
	}))
}

// PrintHelp prints a list of all registered environment variables and their descriptions.
func PrintHelp() {
	log.Print("Environment variables:")
	for _, e := range env {
		log.Printf("  %-40s %s (value: %q)", e.name, e.description, e.value)
	}
}

// HandleHelpFlag looks at the first CLI argument. If it is "help", "-h" or "--help", then it calls
// PrintHelp and exits.
func HandleHelpFlag() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "help", "-h", "--help":
			PrintHelp()
			os.Exit(0)
		}
	}
}
