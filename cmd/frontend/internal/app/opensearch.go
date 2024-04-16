package app

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var openSearchDescription = template.Must(template.New("").Parse(`
<OpenSearchDescription xmlns="http://a9.com/-/spec/opensearch/1.1/" xmlns:moz="http://www.mozilla.org/2006/browser/search/">
  <ShortName>{{.SiteName}}</ShortName>
  <Description>Search {{.SiteName}}</Description>
  <InputEncoding>UTF-8</InputEncoding>
  <Image height="16" width="16">data:image/png;base64,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</Image>
  <Url type="text/html" method="GET" template="{{.SearchURL}}" />
</OpenSearchDescription>
`))

func openSearch(w http.ResponseWriter, r *http.Request) {
	type vars struct {
		SiteName  string
		BaseURL   string
		SearchURL string
	}

	externalURL, err := conf.ExternalURL()
	if err != nil {
		http.Error(w, errors.Wrap(err, "failed to get external URL").Error(), http.StatusInternalServerError)
	}
	externalURLStr := externalURL.String()
	data := vars{
		BaseURL:   externalURLStr,
		SearchURL: externalURLStr + "/search?q={searchTerms}",
	}
	if externalURLStr == "https://sourcegraph.com" {
		data.SiteName = "Sourcegraph"
	} else {
		data.SiteName = "Sourcegraph (" + externalURL.Host + ")"
	}

	var buf bytes.Buffer
	if err := openSearchDescription.Execute(&buf, data); err != nil {
		log15.Error("Failed to execute OpenSearch template", "err", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	_, _ = buf.WriteTo(w)
}
