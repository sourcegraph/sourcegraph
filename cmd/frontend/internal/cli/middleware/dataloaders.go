package middleware

import (
	"fmt"
	"net/http"
)

func DataLoader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// if !isBlackhole(r) {
		// 	next.ServeHTTP(w, r)
		// 	return
		// }

		// trace.SetRouteName(r, "middleware.datalod")
		// w.WriteHeader(http.StatusGone)
		// switch r.URL.Path {
		// case "/healthz", "/__version":
		// 	_, _ = w.Write([]byte(version.Version()))
		// default:
		// 	next.ServeHTTP(w, r)
		// }
		fmt.Println("path is: ", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
