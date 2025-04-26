package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/nixmade/pippy/store"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/google/uuid"
	"github.com/urfave/cli/v3"
)

const (
	AuditPrefix string = "audit:"
)

type Audit struct {
	Time     time.Time
	Resource map[string]string
	Actor    string
	Email    string
	Message  string
}

func Save(ctx context.Context, name string, resource map[string]string, actor, email, msg string) error {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return err
	}
	defer store.Close(dbStore)

	key := fmt.Sprintf("%s%s/%s", AuditPrefix, name, uuid.NewString())

	auditFields := Audit{
		Time:     time.Now().UTC(),
		Resource: resource,
		Actor:    actor,
		Email:    email,
		Message:  msg,
	}
	return dbStore.SaveJSON(key, &auditFields)
}

func Latest(ctx context.Context, name string, resource map[string]string) (*Audit, error) {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer store.Close(dbStore)

	auditKeyPrefix := fmt.Sprintf("%s%s/", AuditPrefix, name)
	var latestAudit Audit
	auditItr := func(key any, value any) error {
		var data Audit
		if err := json.Unmarshal([]byte(value.(string)), &data); err != nil {
			return err
		}
		if !reflect.DeepEqual(data.Resource, resource) {
			return nil
		}
		if data.Time.Compare(latestAudit.Time) >= 0 {
			latestAudit = data
		}
		return nil
	}
	err = dbStore.SortedDescN(auditKeyPrefix, "$.Time", -1, auditItr)
	if err != nil {
		return nil, err
	}

	return &latestAudit, nil
}

func ListAudits(ctx context.Context) (map[string]Audit, error) {
	return ListAuditsN(ctx, -1)
}

func ListAuditsN(ctx context.Context, limit int64) (map[string]Audit, error) {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer store.Close(dbStore)

	audits := make(map[string]Audit)
	auditItr := func(key any, value any) error {
		var data Audit
		if err := json.Unmarshal([]byte(value.(string)), &data); err != nil {
			return err
		}
		audits[key.(string)] = data
		return nil
	}
	err = dbStore.SortedDescN(AuditPrefix, "$.Time", limit, auditItr)
	if err != nil {
		return nil, err
	}
	return audits, nil
}

func ListAuditsUI(limit int64) error {
	audits, err := ListAuditsN(context.Background(), limit)
	if err != nil {
		return err
	}

	rows := [][]string{}
	for auditKey, data := range audits {
		strippedAuditKey, _ := strings.CutPrefix(auditKey, AuditPrefix)

		auditNameId := strings.Split(strippedAuditKey, "/")
		rows = append(rows, []string{data.Time.Format(time.RFC3339), auditNameId[1], auditNameId[0], convertResouceToList(data.Resource), data.Actor, data.Email, data.Message})

	}
	sort.Slice(rows, func(i, j int) bool {
		leftTime, err := time.Parse(time.RFC3339, rows[i][0])
		if err != nil {
			return false
		}

		rightTime, err := time.Parse(time.RFC3339, rows[j][0])
		if err != nil {
			return false
		}

		return leftTime.After(rightTime)
	})

	re := lipgloss.NewRenderer(os.Stdout)

	var (
		// HeaderStyle is the lipgloss style used for the table headers.
		HeaderStyle = re.NewStyle().Foreground(lipgloss.Color("#929292")).Bold(true).Align(lipgloss.Center)
		// CellStyle is the base lipgloss style used for the table rows.
		CellStyle = re.NewStyle().Padding(0, 1).Width(14)
		// OddRowStyle is the lipgloss style used for odd-numbered table rows.
		OddRowStyle = CellStyle.Foreground(lipgloss.Color("#FDFF90"))
		// EvenRowStyle is the lipgloss style used for even-numbered table rows.
		EvenRowStyle = CellStyle.Foreground(lipgloss.Color("#97AD64"))
		// BorderStyle is the lipgloss style used for the table border.
		BorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#97AD64"))
	)

	t := table.New().
		Width(120).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		Headers("TIME", "ID", "TYPE", "RESOURCE", "ACTOR", "EMAIL", "MESSAGE").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == 0:
				return HeaderStyle
			case row%2 == 0:
				return EvenRowStyle
			default:
				return OddRowStyle
			}
		})

	fmt.Println(t)

	return nil
}

func convertResouceToList(resource map[string]string) string {
	var keyValues []string
	for key, value := range resource {
		keyValues = append(keyValues, fmt.Sprintf("%s=%s", key, value))
	}

	return strings.Join(keyValues, ",")
}

func Command() *cli.Command {
	return &cli.Command{
		Name:  "audit",
		Usage: "audit management",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := ListAuditsUI(c.Int64("limit")); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:     "limit",
						Usage:    "audit list limit",
						Value:    int64(10),
						Required: false,
					},
				},
			},
		},
	}
}
