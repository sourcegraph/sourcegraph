package processrestart

import (
	"fmt"
	"net/rpc"
	"os"
)

// usingGoremanServer is whether we are running goreman in cmd/server.
var usingGoremanServer = os.Getenv("GOREMAN_RPC_ADDR") != ""

// restartGoremanServer restarts the processes when running goreman in cmd/server. It takes care to
// avoid a race condition where some services have started up with the new config and some are still
// running with the old config.
func restartGoremanServer() error {
	client, err := rpc.Dial("tcp", os.Getenv("GOREMAN_RPC_ADDR"))
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Call("Goreman.RestartAll", struct{}{}, nil); err != nil {
		return fmt.Errorf("failed to restart all server processes: %s", err)
	}
	return nil
}
