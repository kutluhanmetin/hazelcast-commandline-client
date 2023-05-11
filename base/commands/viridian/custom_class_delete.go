package viridian

import (
	"context"
	"fmt"

	"github.com/hazelcast/hazelcast-commandline-client/clc"
	. "github.com/hazelcast/hazelcast-commandline-client/internal/check"
	"github.com/hazelcast/hazelcast-commandline-client/internal/plug"
)

type CustomClassDeleteCmd struct{}

func (cmd CustomClassDeleteCmd) Init(cc plug.InitContext) error {
	cc.SetCommandUsage("delete-custom-class [cluster-name/cluster-ID] [file-name/artifact-ID] [flags]")
	long := `Deletes a custom class from the given Viridian cluster.

Make sure you login before running this command.
`
	short := "Deletes a custom class from the given Viridian cluster."
	cc.SetCommandHelp(long, short)
	cc.SetPositionalArgCount(2, 2)
	cc.AddStringFlag(propAPIKey, "", "", false, "Viridian API Key")
	return nil
}

func (cmd CustomClassDeleteCmd) Exec(ctx context.Context, ec plug.ExecContext) error {
	api, err := getAPI(ec)
	if err != nil {
		return err
	}
	// inputs
	cluster := ec.Args()[0]
	artifact := ec.Args()[1]
	_, stop, err := ec.ExecuteBlocking(ctx, func(ctx context.Context, sp clc.Spinner) (any, error) {
		sp.SetText("Deleting custom class")
		err = api.DeleteCustomClass(ctx, cluster, artifact)
		if err != nil {
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		ec.Logger().Error(err)
		return fmt.Errorf("deleting custom class. Did you login?: %w", err)
	}
	stop()
	ec.PrintlnUnnecessary("Custom class deleted successfully.")
	return nil
}

func init() {
	Must(plug.Registry.RegisterCommand("viridian:delete-custom-class", &CustomClassDeleteCmd{}))
}
