package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	flag "github.com/spf13/pflag"

	"golang.org/x/net/context"
)

type Args struct {
	Image        string
	Timeout      int
	Showlogs     bool
	Network      string
	RmService    bool
	RegistryCred string
	EnvVars      []string
}

func parseAndValidate() *Args {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	var jaasCmd = &Args{}
	flags.StringSliceVarP(&jaasCmd.EnvVars, "env", "e", nil, "environmental variables")
	flags.StringVar(&jaasCmd.Network, "network", "", "Docker swarm network name")
	flags.BoolVar(&jaasCmd.Showlogs, "showlogs", true, "show logs from stdout")
	flags.BoolVar(&jaasCmd.RmService, "rm", false, "remove service after completion")
	flags.IntVar(&jaasCmd.Timeout, "timeout", 60, "ticks until we time out the service - default is 60 seconds")
	flags.StringVar(&jaasCmd.RegistryCred, "registryAuth", "", "pass your registry authentication")
	flags.Parse(os.Args[1:])
	jaasCmd.Image = flags.Arg(0)

	if jaasCmd.Image == "" {
		panic(fmt.Sprintf("No Image provided"))
	}
	return jaasCmd
}

func main() {
	jaasCmd := parseAndValidate()
	c, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// Check that experimental mode is enabled on the daemon, fall back to no logging if not
	versionInfo, versionErr := c.ServerVersion(context.Background())
	if versionErr != nil {
		panic(versionErr)
	}

	if jaasCmd.Showlogs && versionInfo.Experimental == false {
		panic("daemon required to display service logs, falling back to no log display.")
	}

	spec := makeSpec(jaasCmd.Image, jaasCmd.EnvVars)
	if jaasCmd.Network != "" {
		nets := []swarm.NetworkAttachmentConfig{
			swarm.NetworkAttachmentConfig{Target: jaasCmd.Network},
		}
		spec.Networks = nets
	}

	createOptions := types.ServiceCreateOptions{}

	if jaasCmd.RegistryCred != "" {
		createOptions.EncodedRegistryAuth = jaasCmd.RegistryCred
	}

	createResponse, _ := c.ServiceCreate(context.Background(), spec, createOptions)
	opts := types.ServiceInspectOptions{InsertDefaults: true}

	service, _, _ := c.ServiceInspectWithRaw(context.Background(), createResponse.ID, opts)
	fmt.Printf("Service created: %s (%s)\n", service.Spec.Name, createResponse.ID)

	taskExitCode := pollTask(c, createResponse.ID, jaasCmd.Timeout, jaasCmd.Showlogs, jaasCmd.RmService)
	os.Exit(taskExitCode)

}

func makeSpec(image string, envVars []string) swarm.ServiceSpec {
	max := uint64(1)

	spec := swarm.ServiceSpec{
		TaskTemplate: swarm.TaskSpec{
			RestartPolicy: &swarm.RestartPolicy{
				MaxAttempts: &max,
				Condition:   swarm.RestartPolicyConditionNone,
			},
			ContainerSpec: &swarm.ContainerSpec{
				Image: image,
				Env:   envVars,
			},
		},
	}
	return spec
}
