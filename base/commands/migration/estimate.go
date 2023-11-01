//go:build std || migration

package migration

import (
	"context"
	"fmt"

	"github.com/hazelcast/hazelcast-commandline-client/clc/ux/stage"
	"github.com/hazelcast/hazelcast-commandline-client/internal/check"
	"github.com/hazelcast/hazelcast-commandline-client/internal/plug"
)

type EstimateCmd struct{}

func (e EstimateCmd) Init(cc plug.InitContext) error {
	cc.SetCommandUsage("estimate")
	cc.SetCommandGroup("migration")
	help := "Estimate migration"
	cc.SetCommandHelp(help, help)
	cc.AddStringArg(argDMTConfig, argTitleDMTConfig)
	return nil
}

func (e EstimateCmd) Exec(ctx context.Context, ec plug.ExecContext) error {
	ec.PrintlnUnnecessary("")
	ec.PrintlnUnnecessary(`Hazelcast Data Migration Tool v5.3.0
(c) 2023 Hazelcast, Inc.

Estimation usually ends within 15 seconds.
`)
	mID := MakeMigrationID()
	stages, err := NewEstimateStages(ec.Logger(), mID, ec.GetStringArg(argDMTConfig))
	if err != nil {
		return err
	}
	sp := stage.NewFixedProvider(stages.Build(ctx, ec)...)
	res, err := stage.Execute(ctx, ec, any(nil), sp)
	if err != nil {
		return err
	}
	resArr := res.([]string)
	ec.PrintlnUnnecessary("")
	ec.PrintlnUnnecessary(fmt.Sprintf("OK %s", resArr[0]))
	ec.PrintlnUnnecessary(fmt.Sprintf("OK %s", resArr[1]))
	ec.PrintlnUnnecessary("")
	ec.PrintlnUnnecessary("OK Estimation completed successfully.")
	return nil
}

func init() {
	check.Must(plug.Registry.RegisterCommand("estimate", &EstimateCmd{}))
}
