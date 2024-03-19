package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/schemas"
	"github.com/sourcegraph/sourcegraph/internal/database/postgresdsn"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/highlight"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/symbols"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func printConfigValidation(logger log.Logger) {
	logger = logger.Scoped("configValidation")
	messages, err := conf.Validate(conf.Raw())
	if err != nil {
		logger.Warn("unable to validate Sourcegraph site configuration", log.Error(err))
		return
	}

	if len(messages) > 0 {
		logger.Warn("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
		logger.Warn("⚠️ Warnings related to the Sourcegraph site configuration:")
		for _, verr := range messages {
			logger.Warn(verr.String())
		}
		logger.Warn("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	}
}

var metricConfigOverrideUpdates = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "src_frontend_config_file_watcher_updates",
	Help: "Incremented each time the config file is updated.",
}, []string{"status"})

// readSiteConfigFile reads and merges the paths. paths is the value of the
// envvar SITE_CONFIG_FILE seperated by os.ListPathSeparator (":"). The
// merging just concats the objects together. So does not check for things
// like duplicate keys between files.
func readSiteConfigFile(paths []string) ([]byte, error) {
	// special case 1
	if len(paths) == 1 {
		return os.ReadFile(paths[0])
	}

	var merged bytes.Buffer
	merged.WriteString("// merged SITE_CONFIG_FILE\n{\n")

	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}

		var m map[string]*json.RawMessage
		err = jsonc.Unmarshal(string(b), &m)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse JSON in %s", p)
		}

		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		_, _ = fmt.Fprintf(&merged, "\n// BEGIN %s\n", p)
		for _, k := range keys {
			keyB, _ := json.Marshal(k)
			valB, _ := json.Marshal(m[k])
			_, _ = fmt.Fprintf(&merged, "  %s: %s,\n", keyB, valB)
		}
		_, _ = fmt.Fprintf(&merged, "// END %s\n", p)
	}

	merged.WriteString("}\n")
	formatted, err := jsonc.Format(merged.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to format JSONC")
	}
	return []byte(formatted), nil
}

func overrideSiteConfig(ctx context.Context, logger log.Logger, db database.DB) error {
	logger = logger.Scoped("overrideSiteConfig")
	paths := filepath.SplitList(os.Getenv("SITE_CONFIG_FILE"))
	if len(paths) == 0 {
		return nil
	}
	cs := newConfigurationSource(logger, db)
	updateFunc := func(ctx context.Context) (bool, error) {
		raw, err := cs.Read(ctx)
		if err != nil {
			return false, err
		}
		site, err := readSiteConfigFile(paths)
		if err != nil {
			return false, errors.Wrap(err, "reading SITE_CONFIG_FILE")
		}

		newRawSite := string(site)
		if raw.Site == newRawSite {
			return false, nil
		}

		raw.Site = newRawSite

		// NOTE: authorUserID is effectively 0 because this code is on the start-up path and we will
		// never have a non nil actor available here to determine the user ID. This is consistent
		// with the behaviour of global settings as well. See settings.CreateIfUpToDate in
		// overrideGlobalSettings below.
		//
		// A value of 0 will be treated as null when writing to the the database for this column.
		//
		// Nevertheless, we still use actor.FromContext() because it makes this code future proof in
		// case some how this gets used in a non-startup path as well where an actor is available.
		// In which case we will start populating the authorUserID in the database which is a good
		// thing.
		err = cs.WriteWithOverride(ctx, raw, raw.ID, actor.FromContext(ctx).UID, true)
		if err != nil {
			return false, errors.Wrap(err, "writing site config overrides to database")
		}
		return true, nil
	}
	updated, err := updateFunc(ctx)
	if err != nil {
		return err
	}
	if !updated {
		logger.Info("Site config in critical_and_site_config table is already up to date, skipping writing a new entry")
	}

	go watchUpdate(ctx, logger, updateFunc, paths...)
	return nil
}

func overrideGlobalSettings(ctx context.Context, logger log.Logger, db database.DB) error {
	logger = logger.Scoped("overrideGlobalSettings")
	path := os.Getenv("GLOBAL_SETTINGS_FILE")
	if path == "" {
		return nil
	}
	settings := db.Settings()
	update := func(ctx context.Context) (bool, error) {
		globalSettingsBytes, err := os.ReadFile(path)
		if err != nil {
			return false, errors.Wrap(err, "reading GLOBAL_SETTINGS_FILE")
		}
		currentSettings, err := settings.GetLatest(ctx, api.SettingsSubject{Site: true})
		if err != nil {
			return false, errors.Wrap(err, "could not fetch current settings")
		}
		// Only overwrite the settings if the current settings differ, don't exist, or were
		// created by a human user to prevent creating unnecessary rows in the DB.
		globalSettings := string(globalSettingsBytes)
		if currentSettings == nil || currentSettings.AuthorUserID != nil || currentSettings.Contents != globalSettings {
			var lastID *int32 = nil
			if currentSettings != nil {
				lastID = &currentSettings.ID
			}
			_, err = settings.CreateIfUpToDate(ctx, api.SettingsSubject{Site: true}, lastID, nil, globalSettings)
			if err != nil {
				return false, errors.Wrap(err, "writing global setting override to database")
			}
			return true, nil
		}
		return false, nil
	}
	updated, err := update(ctx)
	if err != nil {
		return err
	}
	if !updated {
		logger.Info("Global settings is already up to date, skipping writing a new entry")
	}

	go watchUpdate(ctx, logger, update, path)

	return nil
}

func overrideExtSvcConfig(ctx context.Context, logger log.Logger, db database.DB) error {
	logger = logger.Scoped("overrideExtSvcConfig")
	path := os.Getenv("EXTSVC_CONFIG_FILE")
	if path == "" {
		return nil
	}
	extsvcs := db.ExternalServices()
	cs := newConfigurationSource(logger, db)

	update := func(ctx context.Context) (bool, error) {
		raw, err := cs.Read(ctx)
		if err != nil {
			return false, err
		}
		parsed, err := conf.ParseConfig(raw)
		if err != nil {
			return false, errors.Wrap(err, "parsing extsvc config")
		}
		confGet := func() *conf.Unified { return parsed }

		extsvcConfig, err := os.ReadFile(path)
		if err != nil {
			return false, errors.Wrap(err, "reading EXTSVC_CONFIG_FILE")
		}
		var rawConfigs map[string][]*json.RawMessage
		if err := jsonc.Unmarshal(string(extsvcConfig), &rawConfigs); err != nil {
			return false, errors.Wrap(err, "parsing EXTSVC_CONFIG_FILE")
		}
		if len(rawConfigs) == 0 {
			logger.Warn("EXTSVC_CONFIG_FILE contains zero external service configurations")
		}

		existing, err := extsvcs.List(ctx, database.ExternalServicesListOptions{})
		if err != nil {
			return false, errors.Wrap(err, "ExternalServices.List")
		}

		// Perform delta update for external services. We don't want to just delete all
		// external services and re-add all of them, because that would cause
		// repo-updater to need to update repositories and reassociate them with external
		// services each time the frontend restarts.
		//
		// Start out by assuming we will remove all and re-add all.
		var (
			toAdd    = make(map[*types.ExternalService]bool)
			toRemove = make(map[*types.ExternalService]bool)
			toUpdate = make(map[int64]*types.ExternalService)
		)
		for _, existing := range existing {
			toRemove[existing] = true
		}
		for key, cfgs := range rawConfigs {
			for i, cfg := range cfgs {
				marshaledCfg, err := json.MarshalIndent(cfg, "", "  ")
				if err != nil {
					return false, errors.Wrapf(err, "marshaling extsvc config ([%v][%v])", key, i)
				}

				// When overriding external service config from a file we allow setting the value
				// of the cloud_default column.
				var cloudDefault bool
				switch key {
				case extsvc.KindGitHub:
					var c schema.GitHubConnection
					if err = json.Unmarshal(marshaledCfg, &c); err != nil {
						return false, err
					}
					cloudDefault = c.CloudDefault

				case extsvc.KindGitLab:
					var c schema.GitLabConnection
					if err = json.Unmarshal(marshaledCfg, &c); err != nil {
						return false, err
					}
					cloudDefault = c.CloudDefault
				}

				toAdd[&types.ExternalService{
					Kind:         key,
					DisplayName:  fmt.Sprintf("%s #%d", key, i+1),
					Config:       extsvc.NewUnencryptedConfig(string(marshaledCfg)),
					CloudDefault: cloudDefault,
				}] = true
			}
		}
		// Now eliminate operations from toAdd/toRemove where the config
		// file and DB describe an equivalent external service.
		isEquiv := func(a, b *types.ExternalService) (bool, error) {
			aConfig, err := a.Config.Decrypt(ctx)
			if err != nil {
				return false, err
			}

			bConfig, err := b.Config.Decrypt(ctx)
			if err != nil {
				return false, err
			}

			return a.Kind == b.Kind && a.DisplayName == b.DisplayName && aConfig == bConfig, nil
		}
		shouldUpdate := func(a, b *types.ExternalService) (bool, error) {
			aConfig, err := a.Config.Decrypt(ctx)
			if err != nil {
				return false, err
			}

			bConfig, err := b.Config.Decrypt(ctx)
			if err != nil {
				return false, err
			}

			return a.Kind == b.Kind && a.DisplayName == b.DisplayName && aConfig != bConfig, nil
		}
		for a := range toAdd {
			for b := range toRemove {
				if ok, err := isEquiv(a, b); err != nil {
					return false, err
				} else if ok {
					// Nothing changed
					delete(toAdd, a)
					delete(toRemove, b)
					continue
				}

				if ok, err := shouldUpdate(a, b); err != nil {
					return false, err
				} else if ok {
					delete(toAdd, a)
					delete(toRemove, b)
					toUpdate[b.ID] = a
				}
			}
		}

		// Apply the delta update.
		for extSvc := range toRemove {
			logger.Debug("Deleting external service", log.Int64("id", extSvc.ID), log.String("displayName", extSvc.DisplayName))
			err := extsvcs.Delete(ctx, extSvc.ID)
			if err != nil {
				return false, errors.Wrap(err, "ExternalServices.Delete")
			}
		}
		for extSvc := range toAdd {
			logger.Debug("Adding external service", log.String("displayName", extSvc.DisplayName))
			if err := extsvcs.Create(ctx, confGet, extSvc); err != nil {
				return false, errors.Wrap(err, "ExternalServices.Create")
			}
		}

		ps := confGet().AuthProviders
		for id, extSvc := range toUpdate {
			logger.Debug("Updating external service", log.Int64("id", id), log.String("displayName", extSvc.DisplayName))

			rawConfig, err := extSvc.Config.Decrypt(ctx)
			if err != nil {
				return false, err
			}
			update := &database.ExternalServiceUpdate{DisplayName: &extSvc.DisplayName, Config: &rawConfig, CloudDefault: &extSvc.CloudDefault}

			if err := extsvcs.Update(ctx, ps, id, update); err != nil {
				return false, errors.Wrap(err, "ExternalServices.Update")
			}
		}
		return true, nil
	}
	updated, err := update(ctx)
	if err != nil {
		return err
	}
	if !updated {
		logger.Info("External site config is already up to date, skipping writing a new entry")
	}

	go watchUpdate(ctx, logger, update, path)
	return nil
}

func watchUpdate(ctx context.Context, logger log.Logger, update func(context.Context) (bool, error), paths ...string) {
	logger = logger.Scoped("watch").With(log.Strings("files", paths))
	events, err := watchPaths(ctx, paths...)
	if err != nil {
		logger.Error("failed to watch config override files", log.Error(err))
		return
	}
	for err := range events {
		if err != nil {
			logger.Warn("error while watching config override files", log.Error(err))
			metricConfigOverrideUpdates.WithLabelValues("watch_failed").Inc()
			continue
		}

		if updated, err := update(ctx); err != nil {
			logger.Error("failed to update configuration from modified config override file", log.Error(err))
			metricConfigOverrideUpdates.WithLabelValues("update_failed").Inc()
		} else if updated {
			logger.Info("updated configuration from modified config override files")
			metricConfigOverrideUpdates.WithLabelValues("success").Inc()
		} else {
			logger.Info("skipped updating configuration as it is already up to date")
			metricConfigOverrideUpdates.WithLabelValues("skipped").Inc()
		}
	}
}

// watchPaths returns a channel which watches the non-empty paths. Whenever
// any path changes a nil error is sent down chan. If an error occurs it is
// sent. chan is closed when ctx is Done.
//
// Note: This can send many events even if the file content hasn't
// changed. For example chmod events are sent. Another is a rename is two
// events for watcher (remove and create). Additionally if a file is removed
// the watch is removed. Even if a file with the same name is created in its
// place later.
func watchPaths(ctx context.Context, paths ...string) (<-chan error, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, p := range paths {
		// as a convenience ignore empty paths
		if p == "" {
			continue
		}
		if err := watcher.Add(p); err != nil {
			return nil, errors.Wrapf(err, "failed to add %s to watcher", p)
		}
	}

	out := make(chan error)
	go func() {
		for {
			select {
			case <-ctx.Done():
				err := watcher.Close()
				if err != nil {
					out <- err
				}
				close(out)
				return

			case <-watcher.Events:
				out <- nil

			case err := <-watcher.Errors:
				out <- err

			}
		}
	}()

	return out, nil
}

func newConfigurationSource(logger log.Logger, db database.DB) *configurationSource {
	return &configurationSource{
		logger: logger.Scoped("configurationSource"),
		db:     db,
	}
}

type configurationSource struct {
	logger log.Logger
	db     database.DB
}

func (c *configurationSource) Read(ctx context.Context) (conftypes.RawUnified, error) {
	site, err := c.db.Conf().SiteGetLatest(ctx)
	if err != nil {
		return conftypes.RawUnified{}, errors.Wrap(err, "ConfStore.SiteGetLatest")
	}

	return conftypes.RawUnified{
		ID:                 site.ID,
		Site:               site.Contents,
		ServiceConnections: serviceConnections(c.logger),
	}, nil
}

func (c *configurationSource) Write(ctx context.Context, input conftypes.RawUnified, lastID int32, authorUserID int32) error {
	return c.WriteWithOverride(ctx, input, lastID, authorUserID, false)
}

func (c *configurationSource) WriteWithOverride(ctx context.Context, input conftypes.RawUnified, lastID int32, authorUserID int32, isOverride bool) error {
	site, err := c.db.Conf().SiteGetLatest(ctx)
	if err != nil {
		return errors.Wrap(err, "ConfStore.SiteGetLatest")
	}
	if site.ID != lastID {
		return errors.New("site config has been modified by another request, write not allowed")
	}
	_, err = c.db.Conf().SiteCreateIfUpToDate(ctx, &site.ID, authorUserID, input.Site, isOverride)
	if err != nil {
		log.Error(errors.Wrap(err, "SiteConfig creation failed"))
		return errors.Wrap(err, "ConfStore.SiteCreateIfUpToDate")
	}
	return nil
}

var (
	serviceConnectionsVal  conftypes.ServiceConnections
	serviceConnectionsOnce sync.Once

	gitserversVal  *endpoint.Map
	gitserversOnce sync.Once
)

func gitservers() *endpoint.Map {
	gitserversOnce.Do(func() {
		addr, err := gitserverAddr(os.Environ())
		if err != nil {
			gitserversVal = endpoint.Empty(errors.Wrap(err, "failed to parse SRC_GIT_SERVERS"))
		} else {
			gitserversVal = endpoint.New(addr)
		}
	})
	return gitserversVal
}

func gitserverAddr(environ []string) (string, error) {
	const (
		serviceName = "gitserver"
		port        = "3178"
	)

	if addr, ok := getEnv(environ, "SRC_GIT_SERVERS"); ok {
		addrs, err := replicaAddrs(deploy.Type(), addr, serviceName, port)
		return addrs, err
	}

	// Detect 'go test' and setup default addresses in that case.
	p, err := os.Executable()
	if err == nil && (strings.HasSuffix(filepath.Base(p), "_test") || strings.HasSuffix(p, ".test")) {
		return "gitserver:3178", nil
	}

	// Not set, use the default (service discovery on searcher)
	return "k8s+rpc://gitserver:3178?kind=sts", nil
}

func serviceConnections(logger log.Logger) conftypes.ServiceConnections {
	serviceConnectionsOnce.Do(func() {
		dsns, err := postgresdsn.DSNsBySchema(schemas.SchemaNames)
		if err != nil {
			panic(err.Error())
		}

		serviceConnectionsVal = conftypes.ServiceConnections{
			PostgresDSN:          dsns["frontend"],
			CodeIntelPostgresDSN: dsns["codeintel"],
			CodeInsightsDSN:      dsns["codeinsights"],
		}
	})

	gitAddrs, err := gitservers().Endpoints()
	if err != nil {
		logger.Error("failed to get gitserver endpoints for service connections", log.Error(err))
	}

	searcherMap := computeSearcherEndpoints()
	searcherAddrs, err := searcherMap.Endpoints()
	if err != nil {
		logger.Error("failed to get searcher endpoints for service connections", log.Error(err))
	}

	symbolsMap := computeSymbolsEndpoints()
	symbolsAddrs, err := symbolsMap.Endpoints()
	if err != nil {
		logger.Error("failed to get symbols endpoints for service connections", log.Error(err))
	}

	zoektMap := computeIndexedEndpoints()
	zoektAddrs, err := zoektMap.Endpoints()
	if err != nil {
		logger.Error("failed to get zoekt endpoints for service connections", log.Error(err))
	}

	embeddingsMap := computeEmbeddingsEndpoints()
	embeddingsAddrs, err := embeddingsMap.Endpoints()
	if err != nil {
		logger.Error("failed to get embeddings endpoints for service connections", log.Error(err))
	}

	return conftypes.ServiceConnections{
		GitServers:           gitAddrs,
		PostgresDSN:          serviceConnectionsVal.PostgresDSN,
		CodeIntelPostgresDSN: serviceConnectionsVal.CodeIntelPostgresDSN,
		CodeInsightsDSN:      serviceConnectionsVal.CodeInsightsDSN,
		Searchers:            searcherAddrs,
		Symbols:              symbolsAddrs,
		Embeddings:           embeddingsAddrs,
		Qdrant:               qdrantAddr,
		Zoekts:               zoektAddrs,
		ZoektListTTL:         indexedListTTL,
	}
}

var (
	searcherURLsOnce sync.Once
	searcherURLs     *endpoint.Map

	symbolsURLsOnce sync.Once
	symbolsURLs     *endpoint.Map

	indexedEndpointsOnce sync.Once
	indexedEndpoints     *endpoint.Map

	embeddingsURLsOnce sync.Once
	embeddingsURLs     *endpoint.Map

	qdrantAddr = os.Getenv("QDRANT_ENDPOINT")

	indexedListTTL = func() time.Duration {
		ttl, _ := time.ParseDuration(env.Get("SRC_INDEXED_SEARCH_LIST_CACHE_TTL", "", "Indexed search list cache TTL"))
		if ttl == 0 {
			if dotcom.SourcegraphDotComMode() {
				ttl = 30 * time.Second
			} else {
				ttl = 5 * time.Second
			}
		}
		return ttl
	}()
)

func computeSymbolsEndpoints() *endpoint.Map {
	symbolsURLsOnce.Do(func() {
		addr, err := symbolsAddr(os.Environ())
		if err != nil {
			symbolsURLs = endpoint.Empty(errors.Wrap(err, "failed to parse SYMBOLS_URL"))
		} else {
			symbolsURLs = endpoint.New(addr)
		}
	})
	return symbolsURLs
}

func symbolsAddr(environ []string) (string, error) {
	const (
		serviceName = "symbols"
		port        = "3184"
	)

	if addr, ok := getEnv(environ, "SYMBOLS_URL"); ok {
		addrs, err := replicaAddrs(deploy.Type(), addr, serviceName, port)
		return addrs, err
	}

	// Not set, use the default (non-service discovery on symbols)
	return "http://symbols:3184", nil
}

func computeEmbeddingsEndpoints() *endpoint.Map {
	embeddingsURLsOnce.Do(func() {
		addr, err := embeddingsAddr(os.Environ())
		if err != nil {
			embeddingsURLs = endpoint.Empty(errors.Wrap(err, "failed to parse EMBEDDINGS_URL"))
		} else {
			embeddingsURLs = endpoint.New(addr)
		}
	})
	return embeddingsURLs
}

func embeddingsAddr(environ []string) (string, error) {
	const (
		serviceName = "embeddings"
		port        = "9991"
	)

	if addr, ok := getEnv(environ, "EMBEDDINGS_URL"); ok {
		addrs, err := replicaAddrs(deploy.Type(), addr, serviceName, port)
		return addrs, err
	}

	// Not set, use the default (non-service discovery on embeddings)
	return "http://embeddings:9991", nil
}

func LoadConfig() {
	highlight.LoadConfig()
	symbols.LoadConfig()
}

func computeSearcherEndpoints() *endpoint.Map {
	searcherURLsOnce.Do(func() {
		addr, err := searcherAddr(os.Environ())
		if err != nil {
			searcherURLs = endpoint.Empty(errors.Wrap(err, "failed to parse SEARCHER_URL"))
		} else {
			searcherURLs = endpoint.New(addr)
		}
	})
	return searcherURLs
}

func searcherAddr(environ []string) (string, error) {
	const (
		serviceName = "searcher"
		port        = "3181"
	)

	if addr, ok := getEnv(environ, "SEARCHER_URL"); ok {
		addrs, err := replicaAddrs(deploy.Type(), addr, serviceName, port)
		return addrs, err
	}

	// Not set, use the default (service discovery on searcher)
	return "k8s+http://searcher:3181", nil
}

func computeIndexedEndpoints() *endpoint.Map {
	indexedEndpointsOnce.Do(func() {
		addr, err := zoektAddr(os.Environ())
		if err != nil {
			indexedEndpoints = endpoint.Empty(errors.Wrap(err, "failed to parse INDEXED_SEARCH_SERVERS"))
		} else {
			if addr != "" {
				indexedEndpoints = endpoint.New(addr)
			} else {
				// It is OK to have no indexed search endpoints.
				indexedEndpoints = endpoint.Static()
			}
		}
	})
	return indexedEndpoints
}

func zoektAddr(environ []string) (string, error) {
	deployType := deploy.Type()

	const port = "6070"
	var baseName = "indexed-search"
	if deployType == deploy.DockerCompose {
		baseName = "zoekt-webserver"
	}

	if addr, ok := getEnv(environ, "INDEXED_SEARCH_SERVERS"); ok {
		addrs, err := replicaAddrs(deployType, addr, baseName, port)
		return addrs, err
	}

	// Backwards compatibility: We used to call this variable ZOEKT_HOST
	if addr, ok := getEnv(environ, "ZOEKT_HOST"); ok {
		return addr, nil
	}

	// Not set, use the default (service discovery on the indexed-search
	// statefulset)
	return "k8s+rpc://indexed-search:6070?kind=sts", nil
}

// Generate endpoints based on replica number when set
func replicaAddrs(deployType, countStr, serviceName, port string) (string, error) {
	count, err := strconv.Atoi(countStr)
	// If countStr is not an int, return string without error
	if err != nil {
		return countStr, nil
	}

	fmtStrHead := ""
	switch serviceName {
	case "searcher", "symbols":
		fmtStrHead = "http://"
	}

	var fmtStrTail string
	switch deployType {
	case deploy.Kubernetes, deploy.Helm, deploy.Kustomize:
		fmtStrTail = fmt.Sprintf(".%s:%s", serviceName, port)
	case deploy.DockerCompose:
		fmtStrTail = fmt.Sprintf(":%s", port)
	default:
		return "", errors.New("Error: unsupported deployment type: " + deployType)
	}

	var addrs []string
	for i := range count {
		addrs = append(addrs, strings.Join([]string{fmtStrHead, serviceName, "-", strconv.Itoa(i), fmtStrTail}, ""))
	}
	return strings.Join(addrs, " "), nil
}

func getEnv(environ []string, key string) (string, bool) {
	key = key + "="
	for _, envVar := range environ {
		if strings.HasPrefix(envVar, key) {
			return envVar[len(key):], true
		}
	}
	return "", false
}
