package rockskip

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
)

// RequestId is a unique int for each HTTP request.
type RequestId = int

// ServiceStatus contains the status of all requests.
type ServiceStatus struct {
	logger                 log.Logger
	threadIdToThreadStatus map[RequestId]*ThreadStatus
	nextThreadId           RequestId
	mu                     sync.Mutex
}

func NewStatus(logger log.Logger) *ServiceStatus {
	return &ServiceStatus{
		logger:                 logger.Scoped("status"),
		threadIdToThreadStatus: map[int]*ThreadStatus{},
		nextThreadId:           0,
		mu:                     sync.Mutex{},
	}
}

func (s *ServiceStatus) NewThreadStatus(name string) *ThreadStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	threadId := s.nextThreadId
	s.nextThreadId++

	threadStatus := NewThreadStatus(name, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.threadIdToThreadStatus, threadId)
	})

	s.threadIdToThreadStatus[threadId] = threadStatus

	return threadStatus
}

func (s *Service) HandleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	repositoryCount, _, err := basestore.ScanFirstInt(s.db.QueryContext(ctx, "SELECT COUNT(*) FROM rockskip_repos"))
	if err != nil {
		s.logger.Error("failed to count repos", log.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type repoRow struct {
		repo           string
		lastAccessedAt time.Time
	}

	repoRows := []repoRow{}
	repoSqlRows, err := s.db.QueryContext(ctx, "SELECT repo, last_accessed_at FROM rockskip_repos ORDER BY last_accessed_at DESC LIMIT 5")
	if err != nil {
		s.logger.Error("failed to list repoRows", log.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer repoSqlRows.Close()
	for repoSqlRows.Next() {
		var repo string
		var lastAccessedAt time.Time
		if err := repoSqlRows.Scan(&repo, &lastAccessedAt); err != nil {
			s.logger.Error("failed to scan repo", log.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		repoRows = append(repoRows, repoRow{repo: repo, lastAccessedAt: lastAccessedAt})
	}

	symbolsSize, _, err := basestore.ScanFirstString(s.db.QueryContext(ctx, "SELECT pg_size_pretty(pg_total_relation_size('rockskip_symbols'))"))
	if err != nil {
		s.logger.Error("failed to get size of symbols table", log.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "This is the symbols service status page.")
	fmt.Fprintln(w, "")

	if os.Getenv("ROCKSKIP_REPOS") != "" {
		fmt.Fprintln(w, "Rockskip is enabled for these repositories:")
		for _, repo := range strings.Split(os.Getenv("ROCKSKIP_REPOS"), ",") {
			fmt.Fprintln(w, "  "+repo)
		}
		fmt.Fprintln(w, "")

		if repositoryCount == 0 {
			fmt.Fprintln(w, "⚠️ None of the enabled repositories have been indexed yet!")
			fmt.Fprintln(w, "⚠️ Open the symbols sidebar on a repository with Rockskip enabled to trigger indexing.")
			fmt.Fprintln(w, "⚠️ Check the logs for errors if requests fail or if there are no in-flight requests below.")
			fmt.Fprintln(w, "⚠️ Docs: https://sourcegraph.com/docs/code_navigation/explanations/rockskip")
			fmt.Fprintln(w, "")
		}
	} else if os.Getenv("ROCKSKIP_MIN_REPO_SIZE_MB") != "" {
		fmt.Fprintf(w, "Rockskip is enabled for repositories over %sMB in size.\n", os.Getenv("ROCKSKIP_MIN_REPO_SIZE_MB"))
		fmt.Fprintln(w, "")
	} else {
		fmt.Fprintln(w, "⚠️ Rockskip is not enabled for any repositories. Remember to set either ROCKSKIP_REPOS or ROCKSKIP_MIN_REPO_SIZE_MB and restart the symbols service.")
		fmt.Fprintln(w, "")
	}

	fmt.Fprintf(w, "Number of rows in rockskip_repos: %d\n", repositoryCount)
	fmt.Fprintf(w, "Size of symbols table: %s\n", symbolsSize)
	fmt.Fprintln(w, "")

	if repositoryCount > 0 {
		fmt.Fprintf(w, "Most recently searched repositories (at most 5 shown)\n")
		for _, repo := range repoRows {
			fmt.Fprintf(w, "  %s %s\n", repo.lastAccessedAt, repo.repo)
		}
		fmt.Fprintln(w, "")
	}

	s.status.mu.Lock()
	defer s.status.mu.Unlock()

	if len(s.status.threadIdToThreadStatus) == 0 {
		fmt.Fprintln(w, "No requests in flight.")
		return
	}
	fmt.Fprintln(w, "Here are all in-flight requests:")
	fmt.Fprintln(w, "")

	ids := []int{}
	for id := range s.status.threadIdToThreadStatus {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		status := s.status.threadIdToThreadStatus[id]
		remaining := status.Remaining()
		status.WithLock(func() {
			fmt.Fprintf(w, "%s\n", status.Name)
			if status.Total > 0 {
				progress := float64(status.Indexed) / float64(status.Total)
				fmt.Fprintf(w, "    progress %.2f%% (indexed %d of %d commits), estimated completion: %s\n", progress*100, status.Indexed, status.Total, remaining)
			}
			fmt.Fprintf(w, "    %s\n", status.Tasklog)
			locks := []string{}
			for lock := range status.HeldLocks {
				locks = append(locks, lock)
			}
			sort.Strings(locks)
			for _, lock := range locks {
				fmt.Fprintf(w, "    holding %s\n", lock)
			}
			fmt.Fprintln(w)
		})
	}
}

type ThreadStatus struct {
	Tasklog   *TaskLog
	Name      string
	HeldLocks map[string]struct{}
	Indexed   int
	Total     int
	mu        sync.Mutex
	onEnd     func()
}

func NewThreadStatus(name string, onEnd func()) *ThreadStatus {
	return &ThreadStatus{
		Tasklog:   NewTaskLog(),
		Name:      name,
		HeldLocks: map[string]struct{}{},
		Indexed:   -1,
		Total:     -1,
		mu:        sync.Mutex{},
		onEnd:     onEnd,
	}
}

func (s *ThreadStatus) WithLock(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f()
}

func (s *ThreadStatus) SetProgress(indexed, total int) {
	s.WithLock(func() { s.Indexed = indexed; s.Total = total })
}
func (s *ThreadStatus) HoldLock(name string)    { s.WithLock(func() { s.HeldLocks[name] = struct{}{} }) }
func (s *ThreadStatus) ReleaseLock(name string) { s.WithLock(func() { delete(s.HeldLocks, name) }) }

func (s *ThreadStatus) End() {
	if s.onEnd != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.onEnd()
	}
}

func (s *ThreadStatus) Remaining() string {
	remaining := "unknown"
	s.WithLock(func() {
		if s.Total > 0 {
			progress := float64(s.Indexed) / float64(s.Total)
			if progress != 0 {
				total := s.Tasklog.TotalDuration()
				remaining = humanize.Time(time.Now().Add(time.Duration(total.Seconds()/progress)*time.Second - total))
			}
		}
	})
	return remaining
}

type TaskLog struct {
	currentName  string
	currentStart time.Time
	nameToTask   map[string]*Task
	// This mutex is only necessary to synchronize with the status page handler.
	mu sync.Mutex
}

type Task struct {
	Duration time.Duration
	Count    int
}

func NewTaskLog() *TaskLog {
	return &TaskLog{
		currentName:  "idle",
		currentStart: time.Now(),
		nameToTask:   map[string]*Task{"idle": {Duration: 0, Count: 1}},
		mu:           sync.Mutex{},
	}
}

func (t *TaskLog) Start(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	if _, ok := t.nameToTask[t.currentName]; !ok {
		t.nameToTask[t.currentName] = &Task{Duration: 0, Count: 0}
	}
	t.nameToTask[t.currentName].Duration += now.Sub(t.currentStart)

	if _, ok := t.nameToTask[name]; !ok {
		t.nameToTask[name] = &Task{Duration: 0, Count: 0}
	}
	t.nameToTask[name].Count += 1

	t.currentName = name
	t.currentStart = now
}

func (t *TaskLog) Continue(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	if _, ok := t.nameToTask[t.currentName]; !ok {
		t.nameToTask[t.currentName] = &Task{Duration: 0, Count: 0}
	}
	t.nameToTask[t.currentName].Duration += now.Sub(t.currentStart)

	if _, ok := t.nameToTask[name]; !ok {
		t.nameToTask[name] = &Task{Duration: 0, Count: 0}
	}

	t.currentName = name
	t.currentStart = now
}

func (t *TaskLog) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.currentName = "idle"
	t.currentStart = time.Now()
	t.nameToTask = map[string]*Task{"idle": {Duration: 0, Count: 1}}
}

func (t *TaskLog) Print() {
	fmt.Println(t)
}

func (t *TaskLog) String() string {
	var s strings.Builder

	t.Continue(t.currentName)

	t.mu.Lock()
	defer t.mu.Unlock()

	var total time.Duration = 0
	totalCount := 0
	for _, task := range t.nameToTask {
		total += task.Duration
		totalCount += task.Count
	}
	fmt.Fprintf(&s, "Tasks (%.2fs total, current %s): ", total.Seconds(), t.currentName)

	type kv struct {
		Key   string
		Value *Task
	}

	var kvs []kv
	for k, v := range t.nameToTask {
		kvs = append(kvs, kv{k, v})
	}

	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].Value.Duration > kvs[j].Value.Duration
	})

	for _, kv := range kvs {
		fmt.Fprintf(&s, "%s %.2f%% %dx, ", kv.Key, kv.Value.Duration.Seconds()*100/total.Seconds(), kv.Value.Count)
	}

	return s.String()
}

func (t *TaskLog) TotalDuration() time.Duration {
	t.Continue(t.currentName)
	var total time.Duration = 0
	for _, task := range t.nameToTask {
		total += task.Duration
	}
	return total
}
