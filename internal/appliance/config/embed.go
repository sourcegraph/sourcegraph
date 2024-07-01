package config

import (
	"embed"
)

var (
	//go:embed postgres/*
	//go:embed prometheus/default.yml.gotmpl
	//go:embed grafana/datasources.yml.gotmpl
	fs embed.FS

	PgsqlConfig                     []byte
	PrometheusDefaultConfigTemplate []byte
	GrafanaDefaultConfigTemplate    []byte
	CodeIntelConfig                 []byte
	CodeInsightsConfig              []byte
)

func init() {
	CodeIntelConfig = mustReadFile("postgres/codeintel.conf")
	CodeInsightsConfig = mustReadFile("postgres/codeinsights.conf")
	PgsqlConfig = mustReadFile("postgres/pgsql.conf")
	PrometheusDefaultConfigTemplate = mustReadFile("prometheus/default.yml.gotmpl")
	GrafanaDefaultConfigTemplate = mustReadFile("grafana/datasources.yml.gotmpl")
}

func mustReadFile(name string) []byte {
	b, err := fs.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return b
}
