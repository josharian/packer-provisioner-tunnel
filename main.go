// Packer-provisioner-tunnel is a packer provisioner plugin.
//
package main

import (
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
	common.PackerConfig `mapstructure:",squash"`

	Exec string   `mapstructure:"exec"`
	Args []string `mapstructure:"args"`

	tpl *packer.ConfigTemplate

	server *sshServer
}

func (t *tunnel) Prepare(raw ...interface{}) error {
	md, err := common.DecodeConfig(t, raw...)
	if err != nil {
		return err
	}
	errs := common.CheckUnusedConfig(md)
	t.tpl, err = packer.NewConfigTemplate()
	if err != nil {
		return err
	}
	t.tpl.UserVars = t.PackerUserVars

	t.Exec, err = t.tpl.Process(t.Exec, nil)
	if err != nil {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("error processing exec template: %s", err))
	}
	if t.Exec == "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("missing tunnel provisioner parameter exec"))
	}

	for i, arg := range t.Args {
		t.Args[i], err = t.tpl.Process(arg, nil)
		if err != nil {
			errs = packer.MultiErrorAppend(errs, fmt.Errorf("error processing arg %d (%q): %s", i, arg, err))
		}
	}

	if errs != nil && len(errs.Errors) > 0 {
		return errs
	}

	var texec string
	texec, err = exec.LookPath(t.Exec)
	if err != nil {
		return fmt.Errorf("executable %q not found: %v", t.Exec, err)
	}
	t.Exec = texec

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
