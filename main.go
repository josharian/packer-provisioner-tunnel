// Packer-provisioner-tunnel is a packer provisioner plugin.
//
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mitchellh/packer/common"
	"github.com/mitchellh/packer/packer"
	"github.com/mitchellh/packer/packer/plugin"
)

type tunnel struct {
	Exec string
	Args []string

	server *sshServer
}

func (t *tunnel) Prepare(raw ...interface{}) error {
	var err error
	_, err = common.DecodeConfig(t, raw...)
	if t.Exec == "" {
		return errors.New("missing exec")
	}
	t.Exec, err = exec.LookPath(t.Exec)
	if err != nil {
		return errors.New("executable " + t.Exec + " not found")
	}

	t.server, err = newSSHServer()
	if err != nil {
		return fmt.Errorf("could not initialize ssh server: %v", err)
	}
	return nil
}

func (t *tunnel) Provision(ui packer.Ui, comm packer.Communicator) error {
	ui.Say("Starting tunnel")
	t.server.comm = comm
	errc := make(chan error, 1)
	go func() {
		errc <- t.server.serveOne()
	}()

	cmd := exec.Command(t.Exec, t.Args...)
	cmd.Env = append(os.Environ(),
		"PACKER_TUNNEL_USERNAME="+t.server.username,
		"PACKER_TUNNEL_PASSWORD="+t.server.password,
		"PACKER_TUNNEL_PORT="+strconv.Itoa(t.server.port),
	)
	log.Println("Command", cmd.Args, "env", cmd.Env)

	ui.Say("Running command " + strings.Join(cmd.Args, " "))
	out, err := cmd.CombinedOutput()
	ui.Say(string(out))
	if err != nil {
		ui.Say(fmt.Sprintf("Error running command %s", err))
		return err
	}

	return <-errc
}

func (t *tunnel) Cancel() {
	log.Println("Cancelled")
	os.Exit(0)
}

func main() {
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}
	server.RegisterProvisioner(new(tunnel))
	server.Serve()
}
