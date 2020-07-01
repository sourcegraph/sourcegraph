package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/inconshreveable/log15"
)

func runCmd(log log15.Logger, errs chan<- error, cmd *exec.Cmd) {
	log.Info(fmt.Sprintf("running: %+v", cmd.Args))
	if err := cmd.Run(); err != nil {
		errs <- fmt.Errorf("command %+v exited: %w", cmd.Args, err)
	}
}

// NewPrometheusCmd instantiates a new command to run Prometheus.
// Parameter promArgs replicates "$@"
func NewPrometheusCmd(promArgs []string, promPort, externalPort string) *exec.Cmd {
	promFlags := []string{
		fmt.Sprintf("--web.listen-address=0.0.0.0:%s", promPort),
	}
	cmd := exec.Command("/prometheus.sh", append(promFlags, promArgs...)...)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd
}

func NewAlertmanagerCmd(configPath string) *exec.Cmd {
	cmd := exec.Command("/alertmanager.sh",
		fmt.Sprintf("--config.file=%s", configPath),
		fmt.Sprintf("--web.route-prefix=/%s", alertmanagerPathPrefix))
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd
}
