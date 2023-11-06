//go:build std || migration

package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/serialization"
	"github.com/hazelcast/hazelcast-go-client/sql"

	"github.com/hazelcast/hazelcast-commandline-client/clc/ux/stage"
	clcerrors "github.com/hazelcast/hazelcast-commandline-client/errors"
	"github.com/hazelcast/hazelcast-commandline-client/internal/plug"
)

var timeoutErr = fmt.Errorf("migration could not be completed: reached timeout while reading status: "+
	"please ensure that you are using Hazelcast's migration cluster distribution and your DMT configuration points to that cluster: %w",
	context.DeadlineExceeded)

var migrationStatusNotFoundErr = fmt.Errorf("migration status not found")

var migrationReportNotFoundErr = "migration report cannot be found: %w"

var noDataStructuresFoundErr = errors.New("no datastructures found to migrate")

var progressMsg = "Migrating %s: %s"

func createMigrationStages(ctx context.Context, ec plug.ExecContext, ci *hazelcast.ClientInternal, migrationID string) ([]stage.Stage[any], error) {
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := WaitForMigrationToBeInProgress(childCtx, ci, migrationID); err != nil {
		return nil, fmt.Errorf("waiting migration to be created: %w", err)
	}
	var stages []stage.Stage[any]
	dss, err := getDataStructuresToBeMigrated(ctx, ec, migrationID)
	if err != nil {
		return nil, err
	}
	for i, d := range dss {
		i := i
		stages = append(stages, stage.Stage[any]{
			ProgressMsg: fmt.Sprintf(progressMsg, d.Type, d.Name),
			SuccessMsg:  fmt.Sprintf("Migrated %s: %s", d.Type, d.Name),
			FailureMsg:  fmt.Sprintf("Failed migrating %s: %s", d.Type, d.Name),
			Func: func(ct context.Context, status stage.Statuser[any]) (any, error) {
				var execErr error
			statusReaderLoop:
				for {
					if ctx.Err() != nil {
						if errors.Is(err, context.DeadlineExceeded) {
							execErr = timeoutErr
							break statusReaderLoop
						}
						execErr = fmt.Errorf("migration failed: %w", err)
						break statusReaderLoop
					}
					generalStatus, err := fetchMigrationStatus(ctx, ci, migrationID)
					if err != nil {
						execErr = fmt.Errorf("reading migration status: %w", err)
						break statusReaderLoop
					}
					switch Status(generalStatus) {
					case StatusStarted:
						break statusReaderLoop
					case StatusComplete:
						return nil, nil
					case StatusFailed:
						errs, err := fetchMigrationErrors(ctx, ci, migrationID)
						if err != nil {
							execErr = fmt.Errorf("fetching migration errors: %w", err)
							break statusReaderLoop
						}
						execErr = errors.New(errs)
						break statusReaderLoop
					case StatusCanceled, StatusCanceling:
						execErr = clcerrors.ErrUserCancelled
						break statusReaderLoop
					case StatusInProgress:
						rt, cp, err := fetchOverallProgress(ctx, ci, migrationID)
						if err != nil {
							ec.Logger().Error(err)
							status.SetText("Unable to calculate remaining duration and progress")
						} else {
							status.SetText(fmt.Sprintf(progressMsg, d.Type, d.Name))
							status.SetProgress(cp)
							status.SetRemainingDuration(rt)
						}
					}
					q := fmt.Sprintf(`SELECT JSON_QUERY(this, '$.migrations[%d]') FROM %s WHERE __key= '%s'`, i, StatusMapName, migrationID)
					res, err := ci.Client().SQL().Execute(ctx, q)
					if err != nil {
						execErr = err
						break statusReaderLoop
					}
					iter, err := res.Iterator()
					if err != nil {
						execErr = err
						break statusReaderLoop
					}
					if iter.HasNext() {
						row, err := iter.Next()
						if err != nil {
							execErr = err
							break statusReaderLoop
						}
						rowStr, err := row.Get(0)
						if err != nil {
							execErr = err
							break statusReaderLoop
						}
						var m DataStructureMigrationStatus
						if err = json.Unmarshal(rowStr.(serialization.JSON), &m); err != nil {
							execErr = err
							break statusReaderLoop
						}
						switch m.Status {
						case StatusComplete:
							return nil, nil
						case StatusFailed:
							return nil, stage.IgnoreError(errors.New(m.Error))
						case StatusCanceled:
							execErr = clcerrors.ErrUserCancelled
							break statusReaderLoop
						}
					}
					time.Sleep(1 * time.Second)
				}
				return nil, execErr
			},
		})
	}
	return stages, nil
}

func getDataStructuresToBeMigrated(ctx context.Context, ec plug.ExecContext, migrationID string) ([]DataStructureInfo, error) {
	var dss []DataStructureInfo
	ci, err := ec.ClientInternal(ctx)
	if err != nil {
		return nil, err
	}
	q := fmt.Sprintf(`SELECT this FROM %s WHERE __key= '%s'`, StatusMapName, migrationID)
	r, err := querySingleRow(ctx, ci, q)
	if err != nil {
		return nil, err
	}
	rr, err := r.Get(0)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, noDataStructuresFoundErr
	}
	var status OverallMigrationStatus
	if err = json.Unmarshal(rr.(serialization.JSON), &status); err != nil {
		return nil, err
	}
	if len(status.Migrations) == 0 {
		return nil, noDataStructuresFoundErr
	}
	for _, m := range status.Migrations {
		dss = append(dss, DataStructureInfo{
			Name: m.Name,
			Type: m.Type,
		})
	}
	return dss, nil
}

func saveMemberLogs(ctx context.Context, ec plug.ExecContext, ci *hazelcast.ClientInternal, migrationID string) error {
	for _, m := range ci.OrderedMembers() {
		l, err := ci.Client().GetList(ctx, DebugLogsListPrefix+m.UUID.String())
		if err != nil {
			return err
		}
		logs, err := l.GetAll(ctx)
		if err != nil {
			return err
		}
		for _, line := range logs {
			ec.Logger().Info(fmt.Sprintf("[%s_%s] %s", migrationID, m.UUID.String(), line.(string)))
		}
	}
	return nil
}

func saveReportToFile(ctx context.Context, ci *hazelcast.ClientInternal, migrationID, fileName string) error {
	report, err := fetchMigrationReport(ctx, ci, migrationID)
	if err != nil {
		return err
	}
	if report == "" {
		return nil
	}
	return os.WriteFile(fileName, []byte(report), 0600)
}

func WaitForMigrationToBeInProgress(ctx context.Context, ci *hazelcast.ClientInternal, migrationID string) error {
	for {
		status, err := fetchMigrationStatus(ctx, ci, migrationID)
		if err != nil {
			if errors.Is(err, migrationStatusNotFoundErr) {
				// migration status will not be available for a while, so we should wait for it
				continue
			}
			return err
		}
		if Status(status) == StatusFailed {
			errs, err := fetchMigrationErrors(ctx, ci, migrationID)
			if err != nil {
				return fmt.Errorf("migration failed and dmt cannot fetch migration errors: %w", err)
			}
			return errors.New(errs)
		}
		if Status(status) == StatusInProgress {
			return nil
		}
	}
}

type OverallMigrationStatus struct {
	Status               Status                         `json:"status"`
	Logs                 []string                       `json:"logs"`
	Errors               []string                       `json:"errors"`
	Report               string                         `json:"report"`
	CompletionPercentage float32                        `json:"completionPercentage"`
	RemainingTime        float32                        `json:"remainingTime"`
	Migrations           []DataStructureMigrationStatus `json:"migrations"`
}

type DataStructureInfo struct {
	Name string
	Type string
}

type DataStructureMigrationStatus struct {
	Name                 string  `json:"name"`
	Type                 string  `json:"type"`
	Status               Status  `json:"status"`
	CompletionPercentage float32 `json:"completionPercentage"`
	Error                string  `json:"error"`
}

func fetchMigrationStatus(ctx context.Context, ci *hazelcast.ClientInternal, migrationID string) (string, error) {
	q := fmt.Sprintf(`SELECT JSON_QUERY(this, '$.status') FROM %s WHERE __key='%s'`, StatusMapName, migrationID)
	r, err := querySingleRow(ctx, ci, q)
	if err != nil {
		return "", migrationStatusNotFoundErr
	}
	rr, err := r.Get(0)
	if err != nil {
		return "", migrationStatusNotFoundErr
	}
	if rr == nil {
		return "", migrationStatusNotFoundErr
	}
	return strings.TrimSuffix(strings.TrimPrefix(string(rr.(serialization.JSON)), `"`), `"`), nil
}

func fetchOverallProgress(ctx context.Context, ci *hazelcast.ClientInternal, migrationID string) (time.Duration, float32, error) {
	q := fmt.Sprintf(`SELECT JSON_QUERY(this, '$.remainingTime'), JSON_QUERY(this, '$.completionPercentage') FROM %s WHERE __key='%s'`, StatusMapName, migrationID)
	r, err := querySingleRow(ctx, ci, q)
	if err != nil {
		return 0, 0, err
	}
	if r == nil {
		return 0, 0, errors.New("overall progress not found")
	}
	remainingTime, err := r.Get(0)
	if err != nil {
		return 0, 0, err
	}
	completionPercentage, err := r.Get(1)
	if err != nil {
		return 0, 0, err
	}
	if completionPercentage == nil {
		return 0, 0, fmt.Errorf("completionPercentage is not available in %s", StatusMapName)
	}
	if remainingTime == nil {
		return 0, 0, fmt.Errorf("remainingTime is not available in %s", StatusMapName)
	}
	rt, err := strconv.ParseInt(remainingTime.(serialization.JSON).String(), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	cpStr := completionPercentage.(serialization.JSON).String()
	cp, err := strconv.ParseFloat(cpStr, 32)
	if err != nil {
		return 0, 0, err
	}
	return time.Duration(rt) * time.Millisecond, float32(cp), nil
}

func fetchMigrationReport(ctx context.Context, ci *hazelcast.ClientInternal, migrationID string) (string, error) {
	q := fmt.Sprintf(`SELECT JSON_QUERY(this, '$.report') FROM %s WHERE __key='%s'`, StatusMapName, migrationID)
	r, err := querySingleRow(ctx, ci, q)
	if err != nil {
		return "", fmt.Errorf(migrationReportNotFoundErr, err)
	}
	if r == nil {
		return "", errors.New("migration report not found")
	}
	rr, err := r.Get(0)
	if err != nil {
		return "", fmt.Errorf(migrationReportNotFoundErr, err)
	}
	var t string
	json.Unmarshal(rr.(serialization.JSON), &t)
	return t, nil
}

func fetchMigrationErrors(ctx context.Context, ci *hazelcast.ClientInternal, migrationID string) (string, error) {
	q := fmt.Sprintf(`SELECT JSON_QUERY(this, '$.errors' WITH WRAPPER) FROM %s WHERE __key='%s'`, StatusMapName, migrationID)
	r, err := querySingleRow(ctx, ci, q)
	if err != nil {
		return "", err
	}
	if r == nil {
		return "", errors.New("could not fetch migration errors")
	}
	rr, err := r.Get(0)
	if err != nil {
		return "", err
	}
	var errs []string
	err = json.Unmarshal(rr.(serialization.JSON), &errs)
	if err != nil {
		return "", err
	}
	return "* " + strings.Join(errs, "\n* "), nil
}

func finalizeMigration(ctx context.Context, ec plug.ExecContext, ci *hazelcast.ClientInternal, migrationID, reportOutputDir string) error {
	err := saveMemberLogs(ctx, ec, ci, migrationID)
	if err != nil {
		return err
	}
	outFile := filepath.Join(reportOutputDir, fmt.Sprintf("migration_report_%s.txt", migrationID))
	err = saveReportToFile(ctx, ci, migrationID, outFile)
	if err != nil {
		return fmt.Errorf("saving report to file: %w", err)
	}
	return nil
}

func querySingleRow(ctx context.Context, ci *hazelcast.ClientInternal, query string) (sql.Row, error) {
	res, err := ci.Client().SQL().Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	it, err := res.Iterator()
	if err != nil {
		return nil, err
	}
	if it.HasNext() {
		// single iteration is enough that we are reading single result for a single migration
		row, err := it.Next()
		if err != nil {
			return nil, err
		}
		return row, nil
	}
	return nil, errors.New("no rows found")
}

func maybePrintWarnings(ctx context.Context, ec plug.ExecContext, ci *hazelcast.ClientInternal, migrationID string) {
	q := fmt.Sprintf(`SELECT JSON_QUERY(this, '$.warnings' WITH WRAPPER) FROM %s WHERE __key='%s'`, StatusMapName, migrationID)
	r, err := querySingleRow(ctx, ci, q)
	if err != nil {
		ec.Logger().Error(err)
		return
	}
	if r == nil {
		ec.Logger().Info("could not find any warnings")
		return
	}
	rr, err := r.Get(0)
	if err != nil {
		ec.Logger().Error(err)
		return
	}
	if rr == nil {
		return
	}
	var warnings []string
	err = json.Unmarshal(rr.(serialization.JSON), &warnings)
	if err != nil {
		ec.Logger().Error(err)
		return
	}
	if len(warnings) <= 5 {
		ec.PrintlnUnnecessary("* " + strings.Join(warnings, "\n* "))
	} else {
		ec.PrintlnUnnecessary(fmt.Sprintf("You have %d warnings that you can find in your migration report.", len(warnings)))
	}
}
