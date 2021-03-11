package app

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/token"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sourcegraph/ctxvfs"
	"golang.org/x/tools/go/buildutil"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/vfsutil"
	"github.com/sourcegraph/sourcegraph/internal/gituri"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"

	"github.com/sourcegraph/go-lsp"

	"github.com/hashicorp/go-multierror"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/gosrc"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
)

// serveGoSymbolURL handles Go symbol URLs (e.g.,
// https://sourcegraph.com/go/github.com/gorilla/mux/-/Vars) by
// redirecting them to the file and line/column URL of the definition.
func serveGoSymbolURL(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid symbol URL path: %q", r.URL.Path)
	}
	mode := parts[0]
	symbolID := strings.Join(parts[1:], "/")

	if mode != "go" {
		return &errcode.HTTPErr{
			Status: http.StatusNotFound,
			Err:    errors.New("invalid mode (only \"go\" is supported"),
		}
	}

	//                                                        def
	//                                                    vvvvvvvvvvvv
	// http://sourcegraph.com/go/github.com/gorilla/mux/-/Router/Match
	//                           ^^^^^^^^^^^^^^^^^^^^^^   ^^^^^^ ^^^^^
	//                                 importPath      receiver? symbolname
	importPath := strings.Split(symbolID, "/-/")[0]
	def := strings.Split(symbolID, "/-/")[1]
	var symbolName string
	var receiver *string
	symbolComponents := strings.Split(def, "/")
	switch len(symbolComponents) {
	case 1:
		symbolName = symbolComponents[0]
	case 2:
		// This is a method call.
		receiver = &symbolComponents[0]
		symbolName = symbolComponents[1]
	default:
		return fmt.Errorf("invalid def %s (must have 1 or 2 path components)", def)
	}

	dir, err := gosrc.ResolveImportPath(httpcli.ExternalDoer(), importPath)
	if err != nil {
		return err
	}
	cloneURL := dir.CloneURL

	if cloneURL == "" || !strings.HasPrefix(cloneURL, "https://github.com") {
		return fmt.Errorf("non-github clone URL resolved for import path %s", importPath)
	}

	repoName := api.RepoName(strings.TrimSuffix(strings.TrimPrefix(cloneURL, "https://"), ".git"))
	repo, err := backend.Repos.GetByName(ctx, repoName)
	if err != nil {
		return err
	}

	commitID, err := backend.Repos.ResolveRev(ctx, repo, "")
	if err != nil {
		return err
	}
	_ = commitID

	vfs, err := repoVFS(r.Context(), repoName, commitID)
	if err != nil {
		return err
	}

	location, err := symbolLocation(r.Context(), vfs, commitID, importPath, path.Join("/", dir.RepoPrefix, strings.TrimPrefix(dir.ImportPath, dir.ProjectRoot)), receiver, symbolName)
	if err != nil {
		return err
	}
	if location == nil {
		return &errcode.HTTPErr{
			Status: http.StatusNotFound,
			Err:    errors.New("symbol not found"),
		}
	}

	uri, err := gituri.Parse(string(location.URI))
	if err != nil {
		return err
	}
	filePath := uri.Fragment
	dest := &url.URL{
		Path:     "/" + path.Join(string(repo.Name), "-/blob", filePath),
		Fragment: fmt.Sprintf("L%d:%d$references", location.Range.Start.Line+1, location.Range.Start.Character+1),
	}
	http.Redirect(w, r, dest.String(), http.StatusFound)
	return nil
}

func symbolLocation(ctx context.Context, vfs ctxvfs.FileSystem, commitID api.CommitID, importPath string, path string, receiver *string, symbol string) (*lsp.Location, error) {
	bctx := buildContextFromVFS(ctx, vfs)

	fileSet := token.NewFileSet()
	pkg, err := parseFiles(fileSet, &bctx, importPath, path)
	if err != nil {
		return nil, err
	}

	pos := (func() *token.Pos {
		docPackage := doc.New(pkg, importPath, doc.AllDecls)
		for _, docConst := range docPackage.Consts {
			for _, spec := range docConst.Decl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					for _, ident := range valueSpec.Names {
						if ident.Name == symbol {
							return &ident.NamePos
						}
					}
				}
			}
		}
		for _, docType := range docPackage.Types {
			if receiver != nil && docType.Name == *receiver {
				for _, method := range docType.Methods {
					if method.Name == symbol {
						return &method.Decl.Name.NamePos
					}
				}
			}
			for _, fun := range docType.Funcs {
				if fun.Name == symbol {
					return &fun.Decl.Name.NamePos
				}
			}
			for _, spec := range docType.Decl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == symbol {
					return &typeSpec.Name.NamePos
				}
			}
		}
		for _, docVar := range docPackage.Vars {
			for _, spec := range docVar.Decl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					for _, ident := range valueSpec.Names {
						if ident.Name == symbol {
							return &ident.NamePos
						}
					}
				}
			}
		}
		for _, docFunc := range docPackage.Funcs {
			if docFunc.Name == symbol {
				return &docFunc.Decl.Name.NamePos
			}
		}
		return nil
	})()

	if pos == nil {
		return nil, nil
	}

	position := fileSet.Position(*pos)
	location := lsp.Location{
		URI: lsp.DocumentURI("https://" + importPath + "?" + string(commitID) + "#" + position.Filename),
		Range: lsp.Range{
			Start: lsp.Position{
				Line:      position.Line - 1,
				Character: position.Column - 1,
			},
			End: lsp.Position{
				Line:      position.Line - 1,
				Character: position.Column - 1,
			},
		},
	}

	return &location, nil
}

func buildContextFromVFS(ctx context.Context, vfs ctxvfs.FileSystem) build.Context {
	bctx := build.Default
	PrepareContext(&bctx, ctx, vfs)
	return bctx
}

func repoVFS(ctx context.Context, name api.RepoName, rev api.CommitID) (ctxvfs.FileSystem, error) {
	if strings.HasPrefix(string(name), "github.com/") {
		return vfsutil.NewGitHubRepoVFS(string(name), string(rev))
	}

	// Fall back to a full git clone for non-github.com repos.
	return nil, fmt.Errorf("unable to fetch repo %s (only github.com repos are supported)", name)
}

func parseFiles(fset *token.FileSet, bctx *build.Context, importPath, srcDir string) (*ast.Package, error) {
	bpkg, err := bctx.ImportDir(srcDir, 0)
	if err != nil {
		return nil, err
	}

	pkg := &ast.Package{
		Files: map[string]*ast.File{},
	}
	var errs error
	for _, file := range append(bpkg.GoFiles, bpkg.TestGoFiles...) {
		if src, err := buildutil.ParseFile(fset, bctx, nil, buildutil.JoinPath(bctx, srcDir), file, 0); err == nil {
			pkg.Name = src.Name.Name
			pkg.Files[file] = src
		} else {
			errs = multierror.Append(errs, err)
		}
	}

	return pkg, errs
}

func PrepareContext(bctx *build.Context, ctx context.Context, fs ctxvfs.FileSystem) {
	// HACK: in the all Context's methods below we are trying to convert path to virtual one (/foo/bar/..)
	// because some code may pass OS-specific arguments.
	// See golang.org/x/tools/go/buildutil/allpackages.go which uses `filepath` for example

	bctx.OpenFile = func(path string) (io.ReadCloser, error) {
		path = filepath.ToSlash(path)
		return fs.Open(ctx, path)
	}
	bctx.IsDir = func(path string) bool {
		path = filepath.ToSlash(path)
		fi, err := fs.Stat(ctx, path)
		return err == nil && fi.Mode().IsDir()
	}
	bctx.HasSubdir = func(root, dir string) (rel string, ok bool) {
		if !bctx.IsDir(dir) {
			return "", false
		}
		if !PathHasPrefix(dir, root) {
			return "", false
		}
		return PathTrimPrefix(dir, root), true
	}
	bctx.ReadDir = func(path string) ([]os.FileInfo, error) {
		path = filepath.ToSlash(path)
		return fs.ReadDir(ctx, path)
	}
	bctx.IsAbsPath = func(path string) bool {
		path = filepath.ToSlash(path)
		return IsAbs(path)
	}
	bctx.JoinPath = func(elem ...string) string {
		// convert all backslashes to slashes to avoid
		// weird paths like C:\mygopath\/src/github.com/...
		for i, el := range elem {
			elem[i] = filepath.ToSlash(el)
		}
		return path.Join(elem...)
	}
}

func trimFilePrefix(s string) string {
	return strings.TrimPrefix(s, "file://")
}

func normalizePath(s string) string {
	if isURI(s) {
		return UriToPath(lsp.DocumentURI(s))
	}
	s = filepath.ToSlash(s)
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return s
}

// PathHasPrefix returns true if s is starts with the given prefix
func PathHasPrefix(s, prefix string) bool {
	s = normalizePath(s)
	prefix = normalizePath(prefix)
	if s == prefix {
		return true
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return s == prefix || strings.HasPrefix(s, prefix)
}

// PathTrimPrefix removes the prefix from s
func PathTrimPrefix(s, prefix string) string {
	s = normalizePath(s)
	prefix = normalizePath(prefix)
	if s == prefix {
		return ""
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return strings.TrimPrefix(s, prefix)
}

// PathEqual returns true if both a and b are equal
func PathEqual(a, b string) bool {
	return PathTrimPrefix(a, b) == ""
}

// IsVendorDir tells if the specified directory is a vendor directory.
func IsVendorDir(dir string) bool {
	return strings.HasPrefix(dir, "vendor/") || strings.Contains(dir, "/vendor/")
}

// IsURI tells if s denotes an URI
func IsURI(s lsp.DocumentURI) bool {
	return isURI(string(s))
}

func isURI(s string) bool {
	return strings.HasPrefix(s, "file://")
}

// PathToURI converts given absolute path to file URI
func PathToURI(path string) lsp.DocumentURI {
	path = filepath.ToSlash(path)
	parts := strings.SplitN(path, "/", 2)

	// If the first segment is a Windows drive letter, prefix with a slash and skip encoding
	head := parts[0]
	if head != "" {
		head = "/" + head
	}

	rest := ""
	if len(parts) > 1 {
		rest = "/" + parts[1]
	}

	return lsp.DocumentURI("file://" + head + rest)
}

// UriToPath converts given file URI to path
func UriToPath(uri lsp.DocumentURI) string {
	u, err := url.Parse(string(uri))
	if err != nil {
		return trimFilePrefix(string(uri))
	}
	return u.Path
}

var regDriveLetter = lazyregexp.New("^/[a-zA-Z]:")

// UriToRealPath converts the given file URI to the platform specific path
func UriToRealPath(uri lsp.DocumentURI) string {
	path := UriToPath(uri)

	if regDriveLetter.MatchString(path) {
		// remove the leading slash if it starts with a drive letter
		// and convert to back slashes
		path = filepath.FromSlash(path[1:])
	}

	return path
}

// IsAbs returns true if the given path is absolute
func IsAbs(path string) bool {
	// Windows implementation accepts path-like and filepath-like arguments
	return strings.HasPrefix(path, "/") || filepath.IsAbs(path)
}

// Panicf takes the return value of recover() and outputs data to the log with
// the stack trace appended. Arguments are handled in the manner of
// fmt.Printf. Arguments should format to a string which identifies what the
// panic code was doing. Returns a non-nil error if it recovered from a panic.
func Panicf(r interface{}, format string, v ...interface{}) error {
	if r != nil {
		// Same as net/http
		const size = 64 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		id := fmt.Sprintf(format, v...)
		log.Printf("panic serving %s: %v\n%s", id, r, string(buf))
		return fmt.Errorf("unexpected panic: %v", r)
	}
	return nil
}
