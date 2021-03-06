package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
	"github.com/docker/swarmkit/cmd/swarmctl/network"
	"github.com/spf13/cobra"
)

var (
	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			var spec *api.ServiceSpec

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}

			if !flags.Changed("name") || !flags.Changed("image") {
				return errors.New("--name and --image are mandatory")
			}
			name, err := flags.GetString("name")
			if err != nil {
				return err
			}

			image, err := flags.GetString("image")
			if err != nil {
				return err
			}

			mode, err := flags.GetString("mode")
			if err != nil {
				return err
			}

			instances, err := flags.GetUint64("instances")
			if err != nil {
				return err
			}

			containerArgs, err := flags.GetStringSlice("args")
			if err != nil {
				return err
			}

			env, err := flags.GetStringSlice("env")
			if err != nil {
				return err
			}

			spec = &api.ServiceSpec{
				Annotations: api.Annotations{
					Name: name,
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{
							Image:   image,
							Command: containerArgs,
							Args:    args,
							Env:     env,
						},
					},
				},
			}

			switch mode {
			case "global":
				spec.Mode = &api.ServiceSpec_Global{}
			case "replicated":
				spec.Mode = &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Instances: instances,
					},
				}
			}

			if flags.Changed("ports") {
				portConfigs, err := flags.GetStringSlice("ports")
				if err != nil {
					return err
				}

				ports := []*api.PortConfig{}
				for _, portConfig := range portConfigs {
					name, protocol, port, swarmPort, err := parsePortConfig(portConfig)
					if err != nil {
						return err
					}

					ports = append(ports, &api.PortConfig{
						Name:      name,
						Protocol:  protocol,
						Port:      port,
						SwarmPort: swarmPort,
					})
				}

				spec.Endpoint = &api.EndpointSpec{
					ExposedPorts: ports,
				}
			}

			if flags.Changed("network") {
				input, err := flags.GetString("network")
				if err != nil {
					return err
				}

				n, err := network.GetNetwork(common.Context(cmd), c, input)
				if err != nil {
					return err
				}

				spec.Networks = []*api.ServiceSpec_NetworkAttachmentConfig{
					{
						Target: n.ID,
					},
				}
			}

			r, err := c.CreateService(common.Context(cmd), &api.CreateServiceRequest{Spec: spec})
			if err != nil {
				return err
			}
			fmt.Println(r.Service.ID)
			return nil
		},
	}
)

func parsePortConfig(portConfig string) (string, api.PortConfig_Protocol, uint32, uint32, error) {
	protocol := api.ProtocolTCP
	parts := strings.Split(portConfig, ":")
	if len(parts) < 2 {
		return "", protocol, 0, 0, fmt.Errorf("insuffient parameters in port configuration")
	}

	name := parts[0]

	portSpec := parts[1]
	protocol, port, err := parsePortSpec(portSpec)
	if err != nil {
		return "", protocol, 0, 0, fmt.Errorf("failed to parse port: %v", err)
	}

	if len(parts) > 2 {
		var err error

		portSpec := parts[2]
		nodeProtocol, swarmPort, err := parsePortSpec(portSpec)
		if err != nil {
			return "", protocol, 0, 0, fmt.Errorf("failed to parse node port: %v", err)
		}

		if nodeProtocol != protocol {
			return "", protocol, 0, 0, fmt.Errorf("protocol mismatch")
		}

		return name, protocol, port, swarmPort, nil
	}

	return name, protocol, port, 0, nil
}

func parsePortSpec(portSpec string) (api.PortConfig_Protocol, uint32, error) {
	parts := strings.Split(portSpec, "/")
	p := parts[0]
	port, err := strconv.ParseUint(p, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	if len(parts) > 1 {
		proto := parts[1]
		protocol, ok := api.PortConfig_Protocol_value[strings.ToUpper(proto)]
		if !ok {
			return 0, 0, fmt.Errorf("invalid protocol string: %s", proto)
		}

		return api.PortConfig_Protocol(protocol), uint32(port), nil
	}

	return api.ProtocolTCP, uint32(port), nil
}

func init() {
	createCmd.Flags().String("name", "", "Service name")
	createCmd.Flags().String("image", "", "Image")
	createCmd.Flags().StringSlice("args", nil, "Args")
	createCmd.Flags().StringSlice("env", nil, "Env")
	createCmd.Flags().StringSlice("ports", nil, "Ports")
	createCmd.Flags().StringP("file", "f", "", "Spec to use")
	createCmd.Flags().String("network", "", "Network name")
	// TODO(aluzzardi): This should be called `service-instances` so that every
	// orchestrator can have its own flag namespace.
	createCmd.Flags().String("mode", "replicated", "one of replicated, global")
	createCmd.Flags().Uint64("instances", 1, "Number of instances for the service Service")
}
