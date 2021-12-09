package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type CommitData struct {
	parent       string
	pathStatuses []PathStatus
}

type MockGit struct {
	commitToCommitData map[string]CommitData
}

func NewMockGit() MockGit {
	return MockGit{
		commitToCommitData: map[string]CommitData{},
	}
}

func (git MockGit) LogReverse(repo string, commit string, n int) ([]LogEntry, error) {
	logEntries := []LogEntry{}
	for commit != "" && n > 0 {
		data, ok := git.commitToCommitData[commit]
		if !ok {
			break
		}
		logEntries = append(logEntries, LogEntry{
			Commit:       commit,
			PathStatuses: data.pathStatuses,
		})
		commit = data.parent
		n -= 1
	}

	// Reverse
	for i, j := 0, len(logEntries)-1; i < j; i, j = i+1, j-1 {
		logEntries[i], logEntries[j] = logEntries[j], logEntries[i]
	}

	return logEntries, nil
}

func (git MockGit) RevList(repo string, commit string) ([]string, error) {
	commits := []string{}
	for commit != "" {
		commits = append(commits, commit)
		commit = git.commitToCommitData[commit].parent
	}
	return commits, nil
}

func RandomCommit() string {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func (git MockGit) AddCommit(commit string, data CommitData) {
	git.commitToCommitData[commit] = data
}

func (git MockGit) ListPaths(commit string) []string {
	paths := map[string]struct{}{}
	commits := []string{}
	current := commit
	for current != "" {
		commits = append(commits, current)
		current = git.commitToCommitData[current].parent
	}
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	for _, commit := range commits {
		for _, pathStatus := range git.commitToCommitData[commit].pathStatuses {
			if pathStatus.Status == AddedAMD {
				paths[pathStatus.Path] = struct{}{}
			} else if pathStatus.Status == DeletedAMD {
				delete(paths, pathStatus.Path)
			}
		}
	}

	pathsSlice := []string{}
	for path := range paths {
		pathsSlice = append(pathsSlice, path)
	}
	sort.Strings(pathsSlice)
	return pathsSlice
}

func (git MockGit) PrintInternals() {
	fmt.Println("Git:")
	fmt.Println()

	for commit, data := range git.commitToCommitData {
		fmt.Println("  commit", commit, "parent", data.parent)
		for _, pathStatus := range data.pathStatuses {
			fmt.Println("    ", statusAMDToString(pathStatus.Status), pathStatus.Path)
		}
	}

	fmt.Println()
}

type MockDB struct {
	commitToHeight         map[string]int
	commitToAncestor       map[string]string
	pathToHopToStatusToIds map[string]map[string]map[StatusAD][]int
	blobs                  map[int]*Blob
}

func NewMockDB() MockDB {
	return MockDB{
		commitToHeight:         map[string]int{},
		commitToAncestor:       map[string]string{},
		pathToHopToStatusToIds: map[string]map[string]map[StatusAD][]int{},
		blobs:                  map[int]*Blob{},
	}
}

func (db MockDB) GetCommit(givenCommit string) (commit string, height int, present bool, err error) {
	height, ok := db.commitToHeight[givenCommit]
	if !ok {
		return "", 0, false, nil
	}
	ancestor, ok := db.commitToAncestor[givenCommit]
	if !ok {
		return "", 0, false, nil
	}

	return ancestor, height, true, nil
}

func (db MockDB) InsertCommit(commit string, height int, ancestor string) error {
	db.commitToHeight[commit] = height
	db.commitToAncestor[commit] = ancestor
	return nil
}

func (db MockDB) GetBlob(hop string, path string) (id int, found bool, err error) {
	hopToStatusToIds, ok := db.pathToHopToStatusToIds[path]
	if !ok {
		return 0, false, nil
	}
	statusToIds, ok := hopToStatusToIds[hop]
	if !ok {
		return 0, false, nil
	}
	addedIds, ok := statusToIds[AddedAD]
	if !ok {
		return 0, false, nil
	}
	addedIdSet := map[int]struct{}{}
	for _, id := range addedIds {
		addedIdSet[id] = struct{}{}
	}
	deletedIds, ok := statusToIds[DeletedAD]
	if ok {
		for _, id := range deletedIds {
			delete(addedIdSet, id)
		}
	}
	for id := range addedIdSet {
		return id, true, nil
	}
	return 0, false, nil
}

func (db MockDB) UpdateBlobHops(id int, status StatusAD, hop string) error {
	if status == AddedAD && !contains(db.blobs[id].added, hop) {
		db.blobs[id].added = append(db.blobs[id].added, hop)
	}
	if status == DeletedAD && !contains(db.blobs[id].deleted, hop) {
		db.blobs[id].deleted = append(db.blobs[id].deleted, hop)
	}

	if _, ok := db.pathToHopToStatusToIds[db.blobs[id].path]; !ok {
		db.pathToHopToStatusToIds[db.blobs[id].path] = map[string]map[StatusAD][]int{}
	}
	if _, ok := db.pathToHopToStatusToIds[db.blobs[id].path][hop]; !ok {
		db.pathToHopToStatusToIds[db.blobs[id].path][hop] = map[StatusAD][]int{}
	}
	db.pathToHopToStatusToIds[db.blobs[id].path][hop][status] = append(db.pathToHopToStatusToIds[db.blobs[id].path][hop][status], id)

	return nil
}

func (db MockDB) InsertBlob(blob Blob) error {
	id := len(db.blobs)
	db.blobs[id] = &blob
	if _, ok := db.pathToHopToStatusToIds[blob.path]; !ok {
		db.pathToHopToStatusToIds[blob.path] = map[string]map[StatusAD][]int{}
	}
	for _, hop := range blob.added {
		if _, ok := db.pathToHopToStatusToIds[blob.path][hop]; !ok {
			db.pathToHopToStatusToIds[blob.path][hop] = map[StatusAD][]int{}
		}
		db.pathToHopToStatusToIds[blob.path][hop][AddedAD] = append(db.pathToHopToStatusToIds[blob.path][hop][AddedAD], id)
	}
	for _, hop := range blob.deleted {
		if _, ok := db.pathToHopToStatusToIds[blob.path][hop]; !ok {
			db.pathToHopToStatusToIds[blob.path][hop] = map[StatusAD][]int{}
		}
		db.pathToHopToStatusToIds[blob.path][hop][DeletedAD] = append(db.pathToHopToStatusToIds[blob.path][hop][DeletedAD], id)
	}
	return nil
}

func (db MockDB) AppendHop(hops []string, givenStatus StatusAD, newHop string) error {
	for _, hop := range hops {
		for _, hopToStatusToIds := range db.pathToHopToStatusToIds {
			for status, ids := range hopToStatusToIds[hop] {
				if status == givenStatus {
					for _, id := range ids {
						db.UpdateBlobHops(id, status, newHop)
					}
				}
			}
		}
	}

	return nil
}

func (db MockDB) Search(hops []string) ([]Blob, error) {
	blobs := []Blob{}
	for _, blob := range db.blobs {
		for _, hop := range hops {
			if contains(blob.deleted, hop) {
				break
			}
			if contains(blob.added, hop) {
				blobs = append(blobs, *blob)
				break
			}
		}
	}
	return blobs, nil
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func (db MockDB) PrintInternals() {
	fmt.Println("Commit ancestry:")
	fmt.Println()

	heights := []int{}
	for _, height := range db.commitToHeight {
		heights = append(heights, height)
	}
	sort.Ints(heights)

	heightToCommits := map[int][]string{}
	for commit, height := range db.commitToHeight {
		if _, ok := heightToCommits[height]; !ok {
			heightToCommits[height] = []string{}
		}
		heightToCommits[height] = append(heightToCommits[height], commit)
	}

	for _, height := range heights {
		for _, commit := range heightToCommits[height] {
			ancestor := db.commitToAncestor[commit]
			ancestorHeight := db.commitToHeight[ancestor]
			fmt.Printf("  - height %d commit %-40s ancestor %-40s (height %d)\n", height, commit, ancestor, ancestorHeight)
		}
	}

	fmt.Println()
	fmt.Println("Blobs:")
	fmt.Println()

	ids := []int{}
	for id := range db.blobs {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		blob := db.blobs[id]
		fmt.Printf("  id %d path %-10s\n", id, blob.path)
		for _, added := range blob.added {
			height := db.commitToHeight[added]
			fmt.Printf("    + %-40s (height %d)\n", added, height)
		}
		fmt.Println()
		for _, deleted := range blob.deleted {
			height := db.commitToHeight[deleted]
			fmt.Printf("    - %-40s (height %d)\n", deleted, height)
		}
		fmt.Println()
	}

	fmt.Println()
}

func TestIndex(t *testing.T) {
	git := NewMockGit()
	db := NewMockDB()

	commits := []string{}
	prevCommit := NULL
	status := AddedAMD
	rand.Seed(0)
	for i := 0; i < 15; i++ {
		commit := RandomCommit()
		commits = append(commits, commit)

		pathStatuses := []PathStatus{}
		if rand.Intn(2) == 0 {
			pathStatuses = append(pathStatuses, PathStatus{Path: "foo.go", Status: status})
			status = invertAMD(status)
		}
		git.AddCommit(commit, CommitData{parent: prevCommit, pathStatuses: pathStatuses})

		prevCommit = commit
	}

	err := Index(git, db, "github.com/foo/bar", prevCommit)
	if err != nil {
		t.Fatalf("🚨 Index: %s", err)
	}

	blobs, err := Search(db, prevCommit)
	if err != nil {
		t.Fatalf("🚨 PathsAtCommit: %s", err)
	}
	paths := []string{}
	for _, blob := range blobs {
		paths = append(paths, blob.path)
	}

	expected := git.ListPaths(prevCommit)

	sort.Strings(paths)
	sort.Strings(expected)

	if diff := cmp.Diff(paths, expected); diff != "" {
		fmt.Println("🚨 PathsAtCommit: unexpected paths (-got +want)", diff)
		git.PrintInternals()
		db.PrintInternals()
		t.Fail()
	}
}

func TestIndexMux(t *testing.T) {
	git := NewSubprocessGit()
	db := NewMockDB()

	repo := "github.com/gorilla/mux"
	head := "3cf0d013e53d62a96c096366d300c84489c26dd5"
	err := Index(git, db, repo, head)
	if err != nil {
		t.Fatalf("🚨 Index: %s", err)
	}

	blobs, err := Search(db, head)
	if err != nil {
		t.Fatalf("🚨 PathsAtCommit: %s", err)
	}
	paths := []string{}
	for _, blob := range blobs {
		paths = append(paths, blob.path)
	}

	cmd := exec.Command("git", "ls-tree", "-r", "--name-only", head)
	cmd.Dir = "/Users/chrismwendt/" + repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")

	sort.Strings(paths)
	sort.Strings(expected)

	if diff := cmp.Diff(paths, expected); diff != "" {
		fmt.Println("🚨 PathsAtCommit: unexpected paths (-got +want)", diff)
		db.PrintInternals()
		t.Fail()
	}
}
