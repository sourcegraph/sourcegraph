package main

import (
	"fmt"
)

const (
	configRepo  = "github.com/go-nacelle/config"
	logRepo     = "github.com/go-nacelle/log"
	nacelleRepo = "github.com/go-nacelle/nacelle"
	navRepo     = "github.com/sourcegraph-testing/nav-test"
	processRepo = "github.com/go-nacelle/process"
	serviceRepo = "github.com/go-nacelle/service"

	configHash  = "72304c5497e662dcf50af212695d2f232b4d32be" // v2.0.0
	logHash     = "b380f4731178f82639695e2a69ae6ec2b8b6dbed" // v2.0.0
	nacelleHash = "05cf7092f82bddbbe0634fa8ca48067bd219a5b5" // v2.0.0
	navHash     = "9156747cf1787b8245f366f81145d565f22c6041" // main
	processHash = "ffadb09a02ca0a8aa6518cf6c118f85ccdc0306c" // v2.0.0
	serviceHash = "ca413da53bba12c23bb73ecf3c7e781664d650e0" // v2.0.0

	//
	// TODO - document this catastrophe :laff:
	numProjects      = 100
	numGroups        = 6
	numLinesPerGroup = 10
	startingLine     = 17
	numGapLines      = 3
)

var testCaseGenerators = []func() []queryFunc{
	testDefsRefsUnexportedSymbol,
	testDefsRefsCrossRepoUseOfFunction,
	testDefsRefsCrossRepoUseOfMethod,
	testDefsRefsMultiRepoUseOfType,
	testDefsRefsMultiRepoUseOfMethod,
	testDefsRefsPagination,
	testProtoImplsCrossRepoInterface,
	testProtoImplsCrossRepoMethod,
	testProtosPagination,
	testImplsPagination,
}

func testDefsRefsUnexportedSymbol() []queryFunc {
	return makeDefsRefsTests(
		"nacelle/logAdapter#",
		[]Location{
			// type struct logAdapter { ... }
			l(nacelleRepo, nacelleHash, "boot.go", 240, 5),
		},
		[]TaggedLocation{
			t(nacelleRepo, nacelleHash, "boot.go", 128, 75, false), // ...(&logAdapter{logger})
			t(nacelleRepo, nacelleHash, "boot.go", 244, 15, false), // func (adapter *logAdapter) ...
			t(nacelleRepo, nacelleHash, "boot.go", 245, 9, false),  // return &logAdapter ...
		},
	)
}

func testDefsRefsCrossRepoUseOfFunction() []queryFunc {
	return makeDefsRefsTests(
		"process/NewContainerBuilder().",
		[]Location{
			l(processRepo, processHash, "container_builder.go", 19, 5), // func NewContainerBuilder() *ContainerBuilder { ... }		},
		},
		[]TaggedLocation{
			t(nacelleRepo, nacelleHash, "boot.go", 123, 36, false), // processContainerBuilder = process.NewContainerBuilder()
		},
	)
}

func testDefsRefsCrossRepoUseOfMethod() []queryFunc {
	return makeDefsRefsTests(
		"log/Logger#Info().",
		[]Location{
			l(logRepo, logHash, "logger.go", 11, 2), // Info(string, ...interface{})
		},
		[]TaggedLocation{
			t(nacelleRepo, nacelleHash, "boot.go", 114, 8, true),           // logger.Info("Logging initialized")
			t(nacelleRepo, nacelleHash, "boot.go", 160, 9, true),           // ... logger.Info("Received signal")
			t(nacelleRepo, nacelleHash, "boot.go", 176, 8, true),           // logger.Info("All processes have stopped")
			t(nacelleRepo, nacelleHash, "config.go", 53, 8, true),          // logger.Info("Loading configuration")
			t(nacelleRepo, nacelleHash, "config.go", 70, 8, true),          // logger.Info("Validating configuration")
			t(nacelleRepo, nacelleHash, "logging_config.go", 21, 11, true), // s.logger.Info(...)
		},
	)
}

func testDefsRefsMultiRepoUseOfType() []queryFunc {
	refs := []TaggedLocation{
		t(logRepo, logHash, "base_logger.go", 28, 77, false),        // func newBaseLogger(...) Logger
		t(logRepo, logHash, "base_logger.go", 40, 111, false),       // func newTestLogger(...) Logger
		t(logRepo, logHash, "emergency.go", 18, 23, false),          // func EmergencyLogger() Logger
		t(logRepo, logHash, "init.go", 10, 28, false),               // func InitLogger(...) (Logger, error)
		t(logRepo, logHash, "logger.go", 4, 33, false),              // WithIndirectCaller(...) Logger
		t(logRepo, logHash, "logger.go", 5, 24, false),              // WithFields(...) Logger
		t(logRepo, logHash, "minimal_logger.go", 22, 45, false),     // func FromMinimalLogger(...) Logger
		t(logRepo, logHash, "minimal_logger.go", 26, 50, false),     // func ... WithIndirectCaller(...) Logger
		t(logRepo, logHash, "minimal_logger.go", 34, 48, false),     // func ... WithFields(...) Logger
		t(logRepo, logHash, "nil_logger.go", 4, 20, false),          // func NewNilLogger() Logger
		t(logRepo, logHash, "replay_logger.go", 103, 38, false),     // func ... record(logger Logger, ...)
		t(logRepo, logHash, "replay_logger.go", 17, 2, false),       // type ReplayLogger interface { ... Logger ... }
		t(logRepo, logHash, "replay_logger.go", 26, 16, false),      // type replayLoggerStruct { ... logger Logger ... }
		t(logRepo, logHash, "replay_logger.go", 31, 2, true),        // type replayLoggerAdapter struct { ... Logger ... }
		t(logRepo, logHash, "replay_logger.go", 44, 10, false),      // type journaledMessage struct { ... logger Logger ... }
		t(logRepo, logHash, "replay_logger.go", 52, 28, false),      // func NewReplayLogger(logger Logger, ...)
		t(logRepo, logHash, "replay_logger.go", 60, 28, false),      // func newReplayLogger(logger Logger, ...)
		t(logRepo, logHash, "rollup_logger.go", 102, 8, false),      // logger Logger,
		t(logRepo, logHash, "rollup_logger.go", 148, 33, false),     // func ... flush(logger Logger)
		t(logRepo, logHash, "rollup_logger.go", 154, 39, false),     // func ... flushLocked(logger Logger)
		t(logRepo, logHash, "rollup_logger.go", 16, 17, false),      // logger Logger
		t(logRepo, logHash, "rollup_logger.go", 38, 28, false),      // func NewRollupLogger(logger Logger, ...)
		t(logRepo, logHash, "rollup_logger.go", 38, 66, false),      // func NewRollupLogger(...) Logger
		t(logRepo, logHash, "rollup_logger.go", 42, 28, false),      // func newRollupLogger(logger Logger, ...)
		t(nacelleRepo, nacelleHash, "boot.go", 241, 5, true),        // type logAdapter struct { ... log.Logger ... }
		t(nacelleRepo, nacelleHash, "log_imports.go", 6, 20, false), // log.Logger
	}

	for p := range numProjects {
		refs = append(refs, t(navRepo, navHash, fmt.Sprintf("proj%d/main.go", p+1), 7, 11, false))

		line := (startingLine - 1)

		for range numGroups {
			refs = append(refs, t(navRepo, navHash, fmt.Sprintf("proj%d/main.go", p+1), line, 16, false))
			line += numLinesPerGroup + numGapLines
		}
	}

	return makeDefsRefsTests(
		"log/Logger#",
		[]Location{
			l(logRepo, logHash, "logger.go", 3, 1), // type Logger interface { ... }
		},
		refs,
	)
}

func testDefsRefsMultiRepoUseOfMethod() []queryFunc {
	return makeDefsRefsTests(
		"service/Container#Set().",
		[]Location{
			l(serviceRepo, serviceHash, "container.go", 52, 20), // func (c *Container) Set ...
		},
		[]TaggedLocation{
			t(nacelleRepo, nacelleHash, "boot.go", 118, 22, false),     // _ = serviceContainer.Set("health", ...)
			t(nacelleRepo, nacelleHash, "boot.go", 119, 22, false),     // _ = serviceContainer.Set("logger", ...)
			t(nacelleRepo, nacelleHash, "boot.go", 120, 22, false),     // _ = serviceContainer.Set("services", ...)
			t(nacelleRepo, nacelleHash, "boot.go", 121, 22, false),     // _ = serviceContainer.Set("config", ...)
			t(serviceRepo, serviceHash, "container.go", 71, 18, false), // return c.parent.Set(...)
			t(serviceRepo, serviceHash, "container.go", 90, 15, false), // if err:= c2.Set(k, v); err != nil { ... }
		},
	)
}

func testDefsRefsPagination() []queryFunc {
	refs := []TaggedLocation{}
	for p := range numProjects {
		line := startingLine

		for range numGroups {
			for range numLinesPerGroup {
				refs = append(refs, t(navRepo, navHash, fmt.Sprintf("proj%d/main.go", p+1), line, 3, false))
				line++
			}

			line += numGapLines
		}
	}

	return makeDefsRefsTests(
		"log/Logger#Warning().",
		[]Location{l(logRepo, logHash, "logger.go", 12, 2)},
		refs,
	)
}

func testProtoImplsCrossRepoInterface() []queryFunc {
	return makeProtoImplsTests(
		"config/Logger#",
		l(configRepo, configHash, "logging_config.go", 12, 5), // type Logger interface { ... }
		[]Location{
			l(configRepo, configHash, "logging_config.go", 17, 5),  // type nilLogger struct{}
			l(nacelleRepo, nacelleHash, "logging_config.go", 4, 5), // type logShim struct{ ... }
		},
	)
}

func testProtoImplsCrossRepoMethod() []queryFunc {
	return makeProtoImplsTests(
		"log/Logger#Info().",
		l(logRepo, logHash, "logger.go", 11, 2), // Info(string, ...interface{})
		[]Location{
			l(logRepo, logHash, "logger.go", 11, 2),          // Info(string, ...interface{})
			l(logRepo, logHash, "minimal_logger.go", 54, 19), // func (sa *adapter) Info(...)
		},
	)
}

func testProtosPagination() (fns []queryFunc) {
	var (
		symbolName = "nav-test/initializerRunnerAndStopper#"
		impls      []Location
		protos     = []Location{
			l(processRepo, processHash, "types.go", 11, 5), // type Initializer interface {
			l(processRepo, processHash, "types.go", 17, 5), // type Runner interface {
			l(processRepo, processHash, "types.go", 25, 5), // type Stopper interface {
		}
	)
	for p := range numProjects {
		impls = append(impls, l(navRepo, navHash, fmt.Sprintf("proj%d/util.go", p+1), 31, 7)) // func (*initializerRunnerAndStopper) Run(...)
	}

	for _, impl := range impls {
		// N.B.: We have multiple prototypes here making the relationship non-symmetric,
		// therefore we're only checking the impl -> proto relationship for this test.
		fns = append(fns, makeProtosTest(symbolName, "implementation", impl, protos))
	}
	return fns
}

func testImplsPagination() (fns []queryFunc) {
	var (
		symbolName = "process/Stopper#"
		proto      = l(processRepo, processHash, "types.go", 25, 5)            // type Stopper interface { ... }
		impls      = []Location{l(processRepo, processHash, "meta.go", 15, 5)} // type Meta struct { ... }
	)
	for p := range numProjects {
		impls = append(impls,
			l(navRepo, navHash, fmt.Sprintf("proj%d/util.go", p+1), 28, 5), // type initializerRunnerAndStopper struct {}
			l(navRepo, navHash, fmt.Sprintf("proj%d/util.go", p+1), 41, 5), // type initializerRunnerStopperAndFinalizer struct {}
		)
	}

	return append(fns,
		// N.B.: These structs implement multiple prototypes making the relationship non-symmetric,
		// therefore we're only checking the proto -> implementations relationship for this test.
		makeImplsTest(symbolName, "prototype", proto, impls),
	)
}
