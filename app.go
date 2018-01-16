package main

import (
	"fmt"
	"os"

	"github.com/docker/cli/cli/command/service"
	"github.com/docker/cli/opts"
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
	Constraints  []string
	Secrets      []string
}

func main() {
	var jaasCmd = &Args{}
	flag.StringSliceVarP(&jaasCmd.EnvVars, "env", "e", nil, "environmental variables")
	flag.StringVar(&jaasCmd.Network, "network", "", "Docker swarm network name")
	flag.BoolVar(&jaasCmd.Showlogs, "showlogs", true, "show logs from stdout")
	flag.BoolVar(&jaasCmd.RmService, "rm", false, "remove service after completion")
	flag.IntVar(&jaasCmd.Timeout, "timeout", 60, "ticks until we time out the service - default is 60 seconds")
	flag.StringVar(&jaasCmd.RegistryCred, "registryAuth", "", "pass your registry authentication")
	flag.StringSliceVar(&jaasCmd.Constraints, "constraint", nil, "Placement constraints (e.g. node.labels.key==value)")
	flag.StringSliceVar(&jaasCmd.Secrets, "secret", nil, "secrets")
	flag.Parse()
	jaasCmd.Image = flag.Arg(0)

	if jaasCmd.Image == "" {
		fmt.Println("No Image provided")
		os.Exit(1)
	}
	c, err := client.NewEnvClient()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	c.NegotiateAPIVersion(context.Background())

	// Check that experimental mode is enabled on the daemon, fall back to no logging if not
	versionInfo, versionErr := c.ServerVersion(context.Background())
	if versionErr != nil {
		fmt.Println(versionErr)
		os.Exit(1)
	}

	if jaasCmd.Showlogs && versionInfo.Experimental == false {
		fmt.Println("Experimental Daemon needed.")
		os.Exit(1)
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

	placement := &swarm.Placement{}
	if jaasCmd.Constraints != nil {
		placement.Constraints = jaasCmd.Constraints
		spec.TaskTemplate.Placement = placement
	}

	if jaasCmd.Secrets != nil {
		var secOpt opts.SecretOpt
		for _, s := range jaasCmd.Secrets {
			secOpt.Set(s)
		}

		if secrets, err := service.ParseSecrets(c, secOpt.Value()); err == nil {
			spec.TaskTemplate.ContainerSpec.Secrets = secrets
		} else {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	createResponse, err := c.ServiceCreate(context.Background(), spec, createOptions)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
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
