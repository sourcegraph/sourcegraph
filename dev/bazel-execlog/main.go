package main

import (
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/sourcegraph/sourcegraph/dev/bazel-execlog/proto"
)

func main() {
	oldf, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer oldf.Close()

	newf, err := os.Open(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer newf.Close()

	oldTargetSpawns := make(Map)
	newTargetSpawns := make(Map)

	r := NewSpawnLogReconstructor(oldf)
	for {
		exec, err := r.GetSpawnExec()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		oldTargetSpawns[exec.TargetLabel] = append(oldTargetSpawns[exec.TargetLabel], exec)
	}

	r = NewSpawnLogReconstructor(newf)
	for {
		exec, err := r.GetSpawnExec()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		newTargetSpawns[exec.TargetLabel] = append(newTargetSpawns[exec.TargetLabel], exec)
	}

	newOnly := newTargetSpawns.Minus(oldTargetSpawns)
	oldOnly := oldTargetSpawns.Minus(newTargetSpawns)

	opts := cmpopts.IgnoreUnexported(proto.SpawnExec{}, proto.EnvironmentVariable{}, proto.Platform{}, proto.Platform_Property{}, proto.File{}, proto.Digest{}, proto.SpawnMetrics{}, durationpb.Duration{}, timestamppb.Timestamp{}, protoimpl.MessageState{})

	// first, show all the entries from the new log that are not in the old log
	for _, v := range newOnly {
		reporter := new(DiffReporter)
		if diff := cmp.Diff(v, []*proto.SpawnExec{}, opts, cmp.Reporter(reporter)); diff != "" {
			fmt.Fprint(os.Stderr, reporter)
		}
	}
	// then, show all the entries from the old log that are not in the new log
	for _, v := range oldOnly {
		reporter := new(DiffReporter)
		if diff := cmp.Diff([]*proto.SpawnExec{}, v, opts, cmp.Reporter(reporter)); diff != "" {
			fmt.Fprint(os.Stderr, reporter)
		}
	}

	// finally, show all the entries that are in both logs, but are different (or not)
	oldShared, newShared := oldTargetSpawns.Intersection(newTargetSpawns)
	reporter := new(DiffReporter)
	if diff := cmp.Diff(oldShared, newShared, opts, cmp.Reporter(reporter)); diff != "" {
		fmt.Fprint(os.Stderr, reporter)
	}
}
