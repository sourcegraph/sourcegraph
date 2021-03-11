package sqliteutil

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"github.com/mattn/go-sqlite3"

	"github.com/sourcegraph/sourcegraph/internal/env"
)

var libSqlite3Pcre = env.Get("LIBSQLITE3_PCRE", "", "path to the libsqlite3-pcre library")

// MustRegisterSqlite3WithPcre registers a sqlite3 driver with PCRE support and
// panics if it can't.
func MustRegisterSqlite3WithPcre() {
	if libSqlite3Pcre == "" {
		env.PrintHelp()
		log.Fatal("can't find the libsqlite3-pcre library because LIBSQLITE3_PCRE was not set")
	}
	sql.Register("sqlite3_with_pcre", &sqlite3.SQLiteDriver{Extensions: []string{libSqlite3Pcre}})
}

// SetLocalLibpath sets the path to the LIBSQLITE3_PCRE shared library. This should
// be called only in test environments. Production environments must require that
// the envvar be set explicitly.
func SetLocalLibpath() {
	if libSqlite3Pcre != "" {
		return
	}

	repositoryRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		panic("can't find the libsqlite3-pcre library because LIBSQLITE3_PCRE was not set and you're not in the git repository, which is where the library is expected to be.")
	}
	if runtime.GOOS == "darwin" {
		libSqlite3Pcre = path.Join(strings.TrimSpace(string(repositoryRoot)), "libsqlite3-pcre.dylib")
	} else {
		libSqlite3Pcre = path.Join(strings.TrimSpace(string(repositoryRoot)), "libsqlite3-pcre.so")
	}
	if _, err := os.Stat(libSqlite3Pcre); os.IsNotExist(err) {
		panic(fmt.Errorf("can't find the libsqlite3-pcre library because LIBSQLITE3_PCRE was not set and %s doesn't exist at the root of the repository - try building it with `./dev/build-libsqlite3pcre.sh`", libSqlite3Pcre))
	}
}
