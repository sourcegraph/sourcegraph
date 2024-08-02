// From https://github.com/buildbuddy-io/buildbuddy-foss/blob/4cacf355631be1e145699bf8614ed9f0665e390a/cli/printlog/compact/compact.go
//
// MIT License

// Copyright (c) 2020-present Iteration, Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"path"
	"slices"

	"google.golang.org/protobuf/encoding/protodelim"

	"github.com/sourcegraph/sourcegraph/dev/bazel-execlog/proto"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// SpawnLogReconstructor reconstructs "compact execution log" format back to the original format.
// As of Bazel 7.1, this is the recommended way to consume the new compact format.
type SpawnLogReconstructor struct {
	input *bufio.Reader

	hashFunc string
	files    map[int32]*proto.File
	dirs     map[int32]*reconstructedDir
	symlinks map[int32]*proto.File
	sets     map[int32]*proto.ExecLogEntry_InputSet
}

type reconstructedDir struct {
	path  string
	files []*proto.File
}

func NewSpawnLogReconstructor(input io.Reader) *SpawnLogReconstructor {
	return &SpawnLogReconstructor{
		input:    bufio.NewReader(input),
		hashFunc: "",
		files:    make(map[int32]*proto.File),
		dirs:     make(map[int32]*reconstructedDir),
		symlinks: make(map[int32]*proto.File),
		sets:     make(map[int32]*proto.ExecLogEntry_InputSet),
	}
}

func (slr *SpawnLogReconstructor) GetSpawnExec() (*proto.SpawnExec, error) {
	entry := &proto.ExecLogEntry{}
	for {
		err := protodelim.UnmarshalOptions{MaxSize: -1}.UnmarshalFrom(slr.input, entry)
		if err == io.EOF {
			return nil, io.EOF
		}
		if err != nil {
			return nil, errors.Newf("failed to read execution log entry: %s", err)
		}

		switch e := entry.GetType().(type) {
		case *proto.ExecLogEntry_Invocation_:
			slr.hashFunc = e.Invocation.GetHashFunctionName()
		case *proto.ExecLogEntry_File_:
			slr.files[entry.GetId()] = reconstructFile(nil, e.File)
		case *proto.ExecLogEntry_Directory_:
			slr.dirs[entry.GetId()] = reconstructDir(e.Directory)
		case *proto.ExecLogEntry_UnresolvedSymlink_:
			slr.symlinks[entry.GetId()] = reconstructSymlink(e.UnresolvedSymlink)
		case *proto.ExecLogEntry_InputSet_:
			slr.sets[entry.GetId()] = e.InputSet
		case *proto.ExecLogEntry_Spawn_:
			return slr.reconstructSpawn(e.Spawn), nil
		default:
			fmt.Printf("unknown exec log entry: %v\n", entry)
		}
	}
}

func (slr *SpawnLogReconstructor) reconstructSpawn(s *proto.ExecLogEntry_Spawn) *proto.SpawnExec {
	se := &proto.SpawnExec{
		CommandArgs:          s.GetArgs(),
		EnvironmentVariables: s.GetEnvVars(),
		TargetLabel:          s.GetTargetLabel(),
		Mnemonic:             s.GetMnemonic(),
		ExitCode:             s.GetExitCode(),
		Status:               s.GetStatus(),
		Runner:               s.GetRunner(),
		CacheHit:             s.GetCacheHit(),
		Remotable:            s.GetRemotable(),
		Cacheable:            s.GetCacheable(),
		RemoteCacheable:      s.GetRemoteCacheable(),
		TimeoutMillis:        s.GetTimeoutMillis(),
		Metrics:              s.GetMetrics(),
		Platform:             s.GetPlatform(),
		Digest:               s.GetDigest(),
	}

	// Handle inputs
	order, inputs := slr.reconstructInputs(s.GetInputSetId())
	_, toolInputs := slr.reconstructInputs(s.GetToolSetId())
	var spawnInputs []*proto.File
	for _, path := range order {
		file := inputs[path]
		if _, ok := toolInputs[path]; ok {
			file.IsTool = true
		}
		spawnInputs = append(spawnInputs, file)
	}
	se.Inputs = spawnInputs
	slices.SortFunc(se.Inputs, func(a, b *proto.File) int {
		return cmp.Compare(a.Path, b.Path)
	})

	// Handle outputs
	var listedOutputs []string
	var actualOutputs []*proto.File
	for _, output := range s.GetOutputs() {
		switch o := output.GetType().(type) {
		case *proto.ExecLogEntry_Output_FileId:
			f := slr.files[o.FileId]
			listedOutputs = append(listedOutputs, f.GetPath())
			actualOutputs = append(actualOutputs, f)
		case *proto.ExecLogEntry_Output_DirectoryId:
			d := slr.dirs[o.DirectoryId]
			listedOutputs = append(listedOutputs, d.path)
			actualOutputs = append(actualOutputs, d.files...)
		case *proto.ExecLogEntry_Output_UnresolvedSymlinkId:
			symlink := slr.symlinks[o.UnresolvedSymlinkId]
			listedOutputs = append(listedOutputs, symlink.GetPath())
			actualOutputs = append(actualOutputs, symlink)
		case *proto.ExecLogEntry_Output_InvalidOutputPath:
			listedOutputs = append(listedOutputs, o.InvalidOutputPath)
		default:
			fmt.Printf("unknown output type: %v\n", output)
		}
	}
	se.ListedOutputs = listedOutputs
	se.ActualOutputs = actualOutputs

	return se
}

func (slr *SpawnLogReconstructor) reconstructInputs(setID int32) ([]string, map[string]*proto.File) {
	var order []string
	inputs := make(map[string]*proto.File)
	setsToVisit := []int32{}
	visited := make(map[int32]struct{})
	if setID != 0 {
		setsToVisit = append(setsToVisit, setID)
		visited[setID] = struct{}{}
	}
	for len(setsToVisit) > 0 {
		currentID := setsToVisit[0]
		setsToVisit = setsToVisit[1:]
		set := slr.sets[currentID]

		for _, fileID := range set.GetFileIds() {
			if _, ok := visited[fileID]; !ok {
				visited[fileID] = struct{}{}
				f := slr.files[fileID]
				order = append(order, f.GetPath())
				inputs[f.GetPath()] = f
			}
		}
		for _, dirID := range set.GetDirectoryIds() {
			if _, ok := visited[dirID]; !ok {
				visited[dirID] = struct{}{}
				d := slr.dirs[dirID]
				for _, f := range d.files {
					order = append(order, f.GetPath())
					inputs[f.GetPath()] = f
				}
			}
		}
		for _, symlinkID := range set.GetUnresolvedSymlinkIds() {
			if _, ok := visited[symlinkID]; !ok {
				visited[symlinkID] = struct{}{}
				s := slr.symlinks[symlinkID]
				order = append(order, s.GetPath())
				inputs[s.GetPath()] = s
			}
		}
		for _, setID := range set.GetTransitiveSetIds() {
			if _, ok := visited[setID]; !ok {
				visited[setID] = struct{}{}
				setsToVisit = append(setsToVisit, setID)
			}
		}
	}
	return order, inputs
}

func reconstructDir(d *proto.ExecLogEntry_Directory) *reconstructedDir {
	filesInDir := make([]*proto.File, 0, len(d.GetFiles()))
	for _, file := range d.GetFiles() {
		filesInDir = append(filesInDir, reconstructFile(d, file))
	}
	return &reconstructedDir{
		path:  d.GetPath(),
		files: filesInDir,
	}
}

func reconstructFile(parentDir *proto.ExecLogEntry_Directory, file *proto.ExecLogEntry_File) *proto.File {
	f := &proto.File{Digest: file.GetDigest()}
	if parentDir != nil {
		f.Path = path.Join(parentDir.GetPath(), file.GetPath())
	} else {
		f.Path = file.GetPath()
	}
	return f
}

func reconstructSymlink(s *proto.ExecLogEntry_UnresolvedSymlink) *proto.File {
	return &proto.File{
		Path:              s.GetPath(),
		SymlinkTargetPath: s.GetTargetPath(),
	}
}
