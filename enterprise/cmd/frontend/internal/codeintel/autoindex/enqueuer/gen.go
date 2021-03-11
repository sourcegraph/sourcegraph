package enqueuer

//go:generate env GOBIN=$PWD/.bin GO111MODULE=on go install github.com/efritz/go-mockgen
//go:generate $PWD/.bin/go-mockgen -f github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codeintel/autoindex/enqueuer -i DBStore -i GitServerClient -i Enqueuer -o mock_iface.go
