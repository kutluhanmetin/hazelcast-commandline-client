package viridian

import (
	"context"
	"fmt"
	"github.com/hazelcast/hazelcast-commandline-client/clc"
	. "github.com/hazelcast/hazelcast-commandline-client/internal/check"
	"github.com/hazelcast/hazelcast-commandline-client/internal/output"
	"github.com/hazelcast/hazelcast-commandline-client/internal/plug"
	"github.com/hazelcast/hazelcast-commandline-client/internal/serialization"
	"github.com/hazelcast/hazelcast-commandline-client/internal/viridian"
)

type CustomClassListCmd struct{}

func (cmd CustomClassListCmd) Init(cc plug.InitContext) error {
	cc.SetCommandUsage("list-custom-classes")
	long := `Lists all Custom Classes in the Cluster.

Make sure you login before running this command.
`
	short := "List Custom Classes in a specific Viridian Cluster"
	cc.SetCommandHelp(long, short)
	cc.SetPositionalArgCount(0, 0)
	cc.AddStringFlag(propAPIKey, "", "", false, "Viridian API Key")

	return nil
}

func (cmd CustomClassListCmd) Exec(ctx context.Context, ec plug.ExecContext) error {
	api, err := getAPI(ec)
	if err != nil {
		return err
	}

	cn := ec.Props().GetString("cluster.name")

	csi, stop, err := ec.ExecuteBlocking(ctx, func(ctx context.Context, sp clc.Spinner) (any, error) {
		sp.SetText("Retrieving custom classes")
		cs, err := api.ListCustomClasses(ctx, cn)
		if err != nil {
			return nil, err
		}
		return cs, nil
	})
	if err != nil {
		ec.Logger().Error(err)
		return fmt.Errorf("error getting custom classes. Did you login?: %w", err)
	}
	stop()

	cs := csi.([]viridian.CustomClass)
	rows := make([]output.Row, len(cs))

	for i, c := range cs {
		rows[i] = output.Row{
			output.Column{
				Name:  "ID",
				Type:  serialization.TypeInt64,
				Value: c.Id,
			},
			output.Column{
				Name:  "Name",
				Type:  serialization.TypeString,
				Value: c.Name,
			},
			output.Column{
				Name:  "GeneratedFileName",
				Type:  serialization.TypeString,
				Value: c.GeneratedFilename,
			},
			output.Column{
				Name:  "Status",
				Type:  serialization.TypeString,
				Value: c.Status,
			},
			output.Column{
				Name:  "TemporaryCustomClassesId",
				Type:  serialization.TypeString,
				Value: c.TemporaryCustomClassesId,
			},
		}
	}
	return ec.AddOutputRows(ctx, rows...)
}

func init() {
	Must(plug.Registry.RegisterCommand("viridian:custom-classes-list", &CustomClassListCmd{}))
}