package ui

import (
	"context"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/golang/gddo/httputil"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
)

// Examples:
//
// Get a plaintext dir listing:
//     http://localhost:3080/github.com/gorilla/mux/-/raw/
//
// Get a file's contents (as text/plain, images will not be rendered by browsers):
//     http://localhost:3080/github.com/gorilla/mux/-/raw/mux.go
//     http://localhost:3080/github.com/sourcegraph/sourcegraph/-/raw/client/web/dist/img/bg-hero.png
//
// Get a zip archive of a repository:
//     curl -H 'Accept: application/zip' http://localhost:3080/github.com/gorilla/mux/-/raw/ -o repo.zip
//
// Get a tar archive of a repository:
//     curl -H 'Accept: application/x-tar' http://localhost:3080/github.com/gorilla/mux/-/raw/ -o repo.tar
//
// Get a zip/tar archive of a _subdirectory_ of a repository:
//     curl -H 'Accept: application/zip' http://localhost:3080/github.com/gorilla/mux/-/raw/.github -o repo-subdir.zip
//
// Get a zip/tar archive of a _file_ in a repository:
//     curl -H 'Accept: application/zip' http://localhost:3080/github.com/gorilla/mux/-/raw/mux.go -o repo-file.zip
//
// Authenticate using an access token:
//     curl -H 'Accept: application/zip' http://fe70a9eeffc8ea7b1edf7c67095c143d1ada7e1b@localhost:3080/github.com/gorilla/mux/-/raw/ -o repo.zip
//
// Download an archive without specifying an Accept header (e.g. download via browser):
//     curl -O -J http://localhost:3080/github.com/gorilla/mux/-/raw?format=zip
//
// Known issues:
//
// - For security reasons, all non-archive files (e.g. code, images, binaries) are served with a Content-Type of text/plain.
// - Symlinks probably do not work well in the text/plain code path (i.e. when not requesting a zip/tar archive).
// - This route would ideally be using strict slashes, in order for us to support symlinks via HTTP redirects.
//

func serveRaw(logger log.Logger, db database.DB, gitserverClient gitserver.Client) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		const (
			textPlain       = "text/plain"
			applicationZip  = "application/zip"
			applicationXTar = "application/x-tar"
		)

		// newCommon provides various repository handling features that we want, so
		// we use it but discard the resulting structure. It provides:
		//
		// - Repo redirection
		// - Gitserver content updating
		// - Consistent error handling (permissions, revision not found, repo not found, etc).
		//
		common, err := newCommon(w, r, db, globals.Branding().BrandName, noIndex, serveError)
		if err != nil {
			return err
		}
		if common == nil {
			return nil // request was handled
		}
		if common.Repo == nil {
			// Repository is cloning.
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", textPlain)
			fmt.Fprintf(w, "Repository unavailable while cloning.")
			return nil
		}

		requestedPath := mux.Vars(r)["Path"]
		if !strings.HasPrefix(requestedPath, "/") {
			requestedPath = "/" + requestedPath
		}

		if requestedPath == "/" && r.Method == "HEAD" {
			_, err := db.Repos().GetByName(r.Context(), common.Repo.Name)
			if err != nil {
				if errcode.IsNotFound(err) {
					w.WriteHeader(http.StatusNotFound)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
				return err
			}
			w.WriteHeader(http.StatusOK)
			return nil
		}

		// Negotiate the content type.
		contentTypeOffers := []string{textPlain, applicationZip, applicationXTar}
		defaultOffer := textPlain
		contentType := httputil.NegotiateContentType(r, contentTypeOffers, defaultOffer)

		// Allow users to override the negotiated content type so that e.g. browser
		// users can easily download tar/zip archives by adding ?format=zip etc. to
		// the URL.
		switch gitserver.ArchiveFormat(r.URL.Query().Get("format")) {
		case gitserver.ArchiveFormatZip:
			contentType = applicationZip
		case gitserver.ArchiveFormatTar:
			contentType = applicationXTar
		}

		// Instrument to understand duration and errors
		var (
			start       = time.Now()
			requestType = "unknown"
			size        int64
		)
		defer func() {
			duration := time.Since(start)
			logger.Debug(
				"raw endpoint",
				log.String("repo", string(common.Repo.Name)),
				log.String("commit", string(common.CommitID)),
				log.String("contentType", contentType),
				log.String("type", requestType),
				log.String("path", requestedPath),
				log.Int64("size", size),
				log.Duration("duration", duration),
				log.Error(err),
			)
			var errorS string
			switch {
			case err == nil:
				errorS = "nil"
			case r.Context().Err() == context.Canceled:
				errorS = "canceled"
			case r.Context().Err() == context.DeadlineExceeded:
				errorS = "timeout"
			default:
				errorS = "error"
			}
			metricRawDuration.WithLabelValues(contentType, requestType, errorS).Observe(duration.Seconds())
		}()

		switch contentType {
		case applicationZip, applicationXTar:
			// Set the proper filename field, so that downloading "/github.com/gorilla/mux/-/raw" gives us a
			// "mux.zip" file (e.g. when downloading via a browser) or a .tar file depending on the contentType.
			ext := ".zip"
			if contentType == applicationXTar {
				ext = ".tar"
			}
			downloadName := path.Base(string(common.Repo.Name)) + ext
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", mime.FormatMediaType("Attachment", map[string]string{"filename": downloadName}))

			format := gitserver.ArchiveFormatZip
			if contentType == applicationXTar {
				format = gitserver.ArchiveFormatTar
			}

			relativePath := strings.TrimPrefix(requestedPath, "/")
			if relativePath == "" {
				relativePath = "."
			}

			if relativePath == "." {
				requestType = "rootarchive"
			} else {
				requestType = "patharchive"
			}

			metricRunning := metricRawArchiveRunning.WithLabelValues(string(format))
			metricRunning.Inc()
			defer metricRunning.Dec()

			// NOTE: we do not use vfsutil since most archives are just streamed once so
			// caching locally is not useful. Additionally we transfer the output over the
			// internet, so we use default compression levels on zips (instead of no
			// compression).
			f, err := gitserverClient.ArchiveReader(r.Context(), common.Repo.ID,
				gitserver.ArchiveOptions{Format: format, Treeish: string(common.CommitID), Paths: []string{relativePath}})
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(w, f)
			return err

		default:
			// This case also applies for defaultOffer. Note that this is preferred
			// over e.g. a 406 status code, according to the MDN:
			// https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/406

			// 🚨 SECURITY: Files are served under the same Sourcegraph domain, and
			// may contain arbitrary contents (JS/HTML files, SVGs with JS in them,
			// malware in the form of .exe, etc). Serving with any other content
			// type is extremely dangerous unless we can guarantee the contents of
			// the file ourselves. GitHub, Wikipedia, and Facebook all use a
			// separate domain for exactly this reason (e.g. raw.githubusercontent.com).
			//
			// See also:
			//
			// - https://security.stackexchange.com/a/11779
			// - https://security.stackexchange.com/a/12916
			// - https://www.owasp.org/index.php/Unrestricted_File_Upload
			// - https://wiki.mozilla.org/WebAppSec/Secure_Coding_Guidelines#Uploads
			//
			// We try to protect against:
			//
			// - Serving user-uploaded malicious JS/HTML, SVGs with JS, etc. in a
			//   browser-interpreted form (not as literal text/plain content),
			//   which would introduce XSS, session-cookie stealing, etc.
			// - Serving user-uploaded malware, etc. which would flag our domain as
			//   untrustworthy by Google, etc. (We do serve such malware, but only
			//   with content type text/plain).
			//
			// We do NOT try to protect against:
			//
			// - Vulnerabilities in old browser versions / old IE versions that do
			//   not respect "nosniff".
			// - Vulnerabilities in Flash or Java (modern browsers should not run
			//   them).
			//
			// Note: We do not use a Content-Disposition attachment here because we
			// want files to be viewed in the browser only AND because doing so
			// would mean that we are literally serving malware to users
			// (i.e. browsers will auto-download it and not treat it as text).
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")

			fi, err := gitserverClient.Stat(r.Context(), common.Repo.Name, common.CommitID, requestedPath)
			if err != nil {
				if os.IsNotExist(err) {
					requestType = "404"
					http.Error(w, html.EscapeString(err.Error()), http.StatusNotFound)
					return nil // request handled
				}
				return err
			}

			if fi.IsDir() {
				requestType = "dir"
				infos, err := gitserverClient.ReadDir(r.Context(), common.Repo.Name, common.CommitID, requestedPath, false)
				if err != nil {
					return err
				}
				size = int64(len(infos))
				var names []string
				for _, info := range infos {
					// A previous version of this code returned relative paths so we trim the paths
					// here too so as not to break backwards compatibility
					name := path.Base(info.Name())
					if info.IsDir() {
						name = name + "/"
					}
					names = append(names, name)
				}
				result := strings.Join(names, "\n")
				fmt.Fprintf(w, "%s", template.HTMLEscapeString(result))
				return nil
			}

			// File
			requestType = "file"
			size = fi.Size()
			f, err := gitserverClient.NewFileReader(r.Context(), common.Repo.Name, common.CommitID, requestedPath)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(w, f)
			return err
		}
	}
}

var metricRawDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "src_frontend_http_raw_duration_seconds",
	Help:    "A histogram of latencies for the raw endpoint.",
	Buckets: prometheus.ExponentialBuckets(.1, 5, 5), // 100ms -> 62s
}, []string{"content", "type", "error"})

var metricRawArchiveRunning = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "src_frontend_http_raw_archive_running",
	Help: "The number of concurrent raw archives being fetched.",
}, []string{"format"})
