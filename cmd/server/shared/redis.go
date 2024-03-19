package shared

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/server/shared/assets"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var redisStoreConfTmpl = template.Must(template.New("redis-store.conf").Parse(assets.RedisStoreConf))
var redisCacheConfTmpl = template.Must(template.New("redis-cache.conf").Parse(assets.RedisCacheConf))

type redisProcfileConfig struct {
	envVar  string
	name    string
	port    string
	tmpl    *template.Template
	dataDir string
}

func maybeRedisStoreProcFile() (string, error) {
	return maybeRedisProcFile(redisProcfileConfig{
		envVar:  "REDIS_STORE_ENDPOINT",
		name:    "redis-store",
		port:    "6379",
		tmpl:    redisStoreConfTmpl,
		dataDir: "redis",
	})
}

func maybeRedisCacheProcFile() (string, error) {
	return maybeRedisProcFile(redisProcfileConfig{
		envVar:  "REDIS_CACHE_ENDPOINT",
		name:    "redis-cache",
		port:    "6380",
		tmpl:    redisCacheConfTmpl,
		dataDir: "redis-cache",
	})
}

func maybeRedisProcFile(c redisProcfileConfig) (string, error) {
	// Redis is already configured. See envvars used in internal/redispool.
	if os.Getenv("REDIS_ENDPOINT") != "" {
		return "", nil
	}

	if os.Getenv(c.envVar) != "" {
		return "", nil
	}

	conf, err := tryCreateRedisConf(c)
	if err != nil {
		return "", err
	}

	if c.dataDir != "" {
		redisFixAOF(os.Getenv("DATA_DIR"), c)
	}

	SetDefaultEnv(c.envVar, "127.0.0.1:"+c.port)

	return redisProcFileEntry(c.name, conf), nil
}

func tryCreateRedisConf(c redisProcfileConfig) (string, error) {
	dataDir := filepath.Join(os.Getenv("DATA_DIR"), c.dataDir)

	var b bytes.Buffer
	err := c.tmpl.Execute(&b, struct{ Dir, Port string }{
		Dir:  dataDir,
		Port: c.port,
	})

	if err != nil {
		return "", err
	}

	err = os.MkdirAll(dataDir, os.FileMode(0755))
	if err != nil {
		return "", err
	}

	// Always replace redis.conf
	path := filepath.Join(os.Getenv("CONFIG_DIR"), c.name+".conf")
	return path, os.WriteFile(path, b.Bytes(), 0644)
}

func redisProcFileEntry(name, conf string) string {
	// Redis is noiser than we prefer even at the most quiet setting "warning"
	// so we only output the last log line when redis stops in case it stopped unexpectly
	// and the log contains the reason why it stopped.
	return name + ": redis-server " + conf + " | tail -n 1"
}

// redisFixAOF does a best-effort repair of the AOF file in case it is
// corrupted https://github.com/sourcegraph/sourcegraph/issues/651
func redisFixAOF(rootDataDir string, c redisProcfileConfig) {
	aofPath := filepath.Join(rootDataDir, c.dataDir, "appendonly.aof")
	if _, err := os.Stat(aofPath); os.IsNotExist(err) {
		return
	}

	done := make(chan struct{})
	go func() {
		var output bytes.Buffer
		e := execer{Out: &output}
		cmd := exec.Command("redis-check-aof", "--fix", aofPath)
		cmd.Stdin = &yesReader{Expletive: []byte("y\n")}
		e.Run(cmd)
		if err := e.Error(); err != nil {
			pgPrintf("Repairing %s appendonly.aof failed:\n%s", c.name, output.String())
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		pgPrintf("Running redis-check-aof --fix %q...", aofPath)
		<-done
		pgPrintf("Finished running redis-check-aof")
	}
}

// yesReader simulates the output of the "yes" command.
//
// It is equivalent to bytes.NewReader(bytes.Repeat(Expletive, infinity))
type yesReader struct {
	Expletive []byte
	offset    int
}

func (r *yesReader) Read(p []byte) (int, error) {
	if len(r.Expletive) == 0 {
		return 0, errors.New("yesReader.Expletive is empty")
	}
	for n := range len(p) {
		p[n] = r.Expletive[r.offset]
		r.offset++
		if r.offset == len(r.Expletive) {
			r.offset = 0
		}
	}
	return len(p), nil
}
