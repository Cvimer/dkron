package dcron

import (
	"fmt"
	"github.com/hashicorp/serf/client"
	serfs "github.com/hashicorp/serf/serf"
	"os"
	"os/exec"
	"strings"
	"time"
)

var serf *client.RPCClient

// spawn command that specified as proc.
func spawnProc(proc string) error {
	cs := []string{"/bin/bash", "-c", proc}
	cmd := exec.Command(cs[0], cs[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	cmd.Env = append(os.Environ())

	fmt.Fprintf(log.Writer(), "Starting %s\n", proc)
	err := cmd.Start()
	if err != nil {
		fmt.Fprintf(log.Writer(), "Failed to start %s: %s\n", proc, err)
		return err
	}
	return cmd.Wait()
}

func InitSerfAgent() {
	discover := ""
	if config.GetString("discover") != "" {
		discover = " -discover=" + config.GetString("discover")
	}
	serfArgs := []string{discover, "-rpc-addr=" + config.GetString("rpc_addr"), "-config-file=config/dcron.json"}
	go spawnProc("./bin/serf agent" + strings.Join(serfArgs, " "))
	serf = initSerfClient()
	defer serf.Close()
	ch := make(chan map[string]interface{}, 1)

	sh, err := serf.Stream("*", ch)
	if err != nil {
		log.Error(err)
	}
	defer serf.Stop(sh)

	for {
		select {
		case event := <-ch:
			for key, val := range event {
				switch ev := val.(type) {
				case serfs.MemberEvent:
					log.Debug(ev)
				default:
					log.Debugf("Receiving event: %s => %v of type %T", key, val, val)
				}
			}
			if event["Event"] == "query" {
				log.Debug(string(event["Payload"].([]byte)))
				serf.Respond(uint64(event["ID"].(int64)), []byte("Peetttee"))
			}
		}
	}
}

func initSerfClient() *client.RPCClient {
	serfConfig := &client.Config{Addr: config.GetString("rpc_addr")}
	serfClient, err := client.ClientFromConfig(serfConfig)
	// wait for serf
	for i := 0; err != nil && i < 5; i = i + 1 {
		log.Debug(err)
		time.Sleep(1 * time.Second)
		serfClient, err = client.ClientFromConfig(serfConfig)
		log.Debugf("Connect to serf agent retry: %d", i)
	}
	if err != nil {
		log.Error("Error connecting to serf instance", err)
		return nil
	}
	return serfClient
}

type Event struct {
	Event   string
	ID      int
	LTime   uint64
	Name    string
	Payload []byte
}
