package internal

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/executil"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git/gitcli"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestServer_handleP4Exec(t *testing.T) {
	defaultMockRunCommand := func(ctx context.Context, cmd *exec.Cmd) (int, error) {
		switch cmd.Args[1] {
		case "users":
			_, _ = cmd.Stdout.Write([]byte("admin <admin@joe-perforce-server> (admin) accessed 2021/01/31"))
			_, _ = cmd.Stderr.Write([]byte("teststderr"))
			return 42, errors.New("the answer to life the universe and everything")
		}
		return 0, nil
	}

	t.Cleanup(func() {
		executil.UpdateRunCommandMock(nil)
	})

	startServer := func(t *testing.T) (handler http.Handler, client proto.GitserverServiceClient, cleanup func()) {
		t.Helper()

		logger := logtest.Scoped(t)

		s := &Server{
			Logger:                  logger,
			ReposDir:                t.TempDir(),
			ObservationCtx:          observation.TestContextTB(t),
			skipCloneForTests:       true,
			DB:                      dbmocks.NewMockDB(),
			RecordingCommandFactory: wrexec.NewNoOpRecordingCommandFactory(),
			Locker:                  NewRepositoryLocker(),
			GetBackendFunc: func(dir common.GitDir, repoName api.RepoName) (git.GitBackend, error) {
				return gitcli.NewBackend(logtest.Scoped(t), wrexec.NewNoOpRecordingCommandFactory(), dir, repoName), nil
			},
		}

		server := defaults.NewServer(logger)
		proto.RegisterGitserverServiceServer(server, NewGRPCServer(s))
		handler = grpc.MultiplexHandlers(server, s.Handler())

		srv := httptest.NewServer(handler)

		u, _ := url.Parse(srv.URL)
		conn, err := defaults.Dial(u.Host, logger.Scoped("gRPC client"))
		if err != nil {
			t.Fatalf("failed to dial: %v", err)
		}

		client = proto.NewGitserverServiceClient(conn)

		return handler, client, func() {
			srv.Close()
			conn.Close()
			server.Stop()
		}
	}

	readAll := func(execClient proto.GitserverService_P4ExecClient) ([]byte, error) {
		var buf bytes.Buffer
		for {
			resp, err := execClient.Recv()
			if errors.Is(err, io.EOF) {
				return buf.Bytes(), nil
			}

			if err != nil {
				return buf.Bytes(), err
			}

			_, err = buf.Write(resp.GetData())
			if err != nil {
				t.Fatalf("failed to write data: %v", err)
			}
		}
	}

	t.Run("users", func(t *testing.T) {
		executil.UpdateRunCommandMock(defaultMockRunCommand)

		_, client, closeFunc := startServer(t)
		t.Cleanup(closeFunc)

		stream, err := client.P4Exec(context.Background(), &proto.P4ExecRequest{
			Args: [][]byte{[]byte("users")},
		})
		if err != nil {
			t.Fatalf("failed to call P4Exec: %v", err)
		}

		data, err := readAll(stream)
		s, ok := status.FromError(err)
		if !ok {
			t.Fatal("received non-status error from p4exec call")
		}

		if diff := cmp.Diff("the answer to life the universe and everything", s.Message()); diff != "" {
			t.Fatalf("unexpected error in stream (-want +got):\n%s", diff)
		}

		expectedData := []byte("admin <admin@joe-perforce-server> (admin) accessed 2021/01/31")

		if diff := cmp.Diff(expectedData, data); diff != "" {
			t.Fatalf("unexpected data (-want +got):\n%s", diff)
		}
	})

	t.Run("empty request", func(t *testing.T) {
		executil.UpdateRunCommandMock(defaultMockRunCommand)

		_, client, closeFunc := startServer(t)
		t.Cleanup(closeFunc)

		stream, err := client.P4Exec(context.Background(), &proto.P4ExecRequest{})
		if err != nil {
			t.Fatalf("failed to call P4Exec: %v", err)
		}

		_, err = readAll(stream)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument error, got %v", err)
		}
	})

	t.Run("disallowed command", func(t *testing.T) {

		executil.UpdateRunCommandMock(defaultMockRunCommand)

		_, client, closeFunc := startServer(t)
		t.Cleanup(closeFunc)

		stream, err := client.P4Exec(context.Background(), &proto.P4ExecRequest{
			Args: [][]byte{[]byte("bad_command")},
		})
		if err != nil {
			t.Fatalf("failed to call P4Exec: %v", err)
		}

		_, err = readAll(stream)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument error, got %v", err)
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		executil.UpdateRunCommandMock(func(ctx context.Context, _ *exec.Cmd) (int, error) {
			// fake a context cancellation that occurs while the process is running

			cancel()
			return 0, ctx.Err()
		})

		_, client, closeFunc := startServer(t)
		t.Cleanup(closeFunc)

		stream, err := client.P4Exec(ctx, &proto.P4ExecRequest{
			Args: [][]byte{[]byte("users")},
		})
		if err != nil {
			t.Fatalf("failed to call P4Exec: %v", err)
		}

		_, err = readAll(stream)
		if !(errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled) {
			t.Fatalf("expected context cancelation error, got %v", err)
		}
	})
}
