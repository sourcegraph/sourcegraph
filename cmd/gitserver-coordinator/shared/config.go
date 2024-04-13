package shared

import (
	"net"

	"github.com/sourcegraph/sourcegraph/internal/env"
)

func LoadConfig() *Config {
	var config Config
	config.Load()
	return &config
}

type Config struct {
	env.BaseConfig

	ListenAddress    string
	HTTPCloneAddress string
}

func (c *Config) Load() {
	c.ListenAddress = c.GetOptional("GITSERVER_COORDINATOR_ADDR", "The address under which the gitserver coordinator Git API listens. Can include a port.")
	// Fall back to a reasonable default.
	if c.ListenAddress == "" {
		port := "3199"
		host := ""
		if env.InsecureDev {
			host = "127.0.0.1"
		}
		c.ListenAddress = net.JoinHostPort(host, port)
	}
	c.HTTPCloneAddress = c.GetOptional("GITSERVER_COORDINATOR_HTTP_CLONE_ADDR", "The address under which the gitserver coordinator HTTP clone API listens. Can include a port.")
	// Fall back to a reasonable default.
	if c.HTTPCloneAddress == "" {
		port := "3213"
		host := ""
		if env.InsecureDev {
			host = "127.0.0.1"
		}
		c.HTTPCloneAddress = net.JoinHostPort(host, port)
	}
}
