package resources

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/episki/episki-cli/internal/auth"
	"github.com/spf13/cobra"
	"github.com/supabase-community/postgrest-go"
)

func workItemsCmd(rf *auth.RootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "work-items",
		Aliases: []string{"wi", "tasks"},
		Short:   "Work items (tasks, issues, reviews, ...) in the workspace",
	}
	cmd.AddCommand(workItemsListCmd(rf))
	cmd.AddCommand(workItemsGetCmd(rf))
	cmd.AddCommand(workItemsUpdateCmd(rf))
	cmd.AddCommand(workItemsArchiveCmd(rf, true))
	cmd.AddCommand(workItemsArchiveCmd(rf, false))
	return cmd
}

const (
	workItemListSelect = "id,ref,name,kind,priority,due_at,updated_at,statuses(name,category)"
	workItemGetSelect  = "id,ref,name,kind,priority,due_at,completed_at,archived_at,owner_id," +
		"responsible_owner_id,parent_id,status_id,status_group_id,created_at,updated_at,statuses(name,category)"
)

var workItemCols = []column{
	{"id", "id"}, {"ref", "ref"}, {"name", "name"}, {"kind", "kind"},
	{"status", "statuses.name"}, {"priority", "priority"},
	{"due", "due_at"}, {"updated", "updated_at"},
}

func workItemsListCmd(rf *auth.RootFlags) *cobra.Command {
	var (
		lf       listFlags
		kind     string
		archived bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List work items in the active workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := connect(cmd.Context(), rf)
			if err != nil {
				return err
			}
			ws, err := c.requireWorkspace()
			if err != nil {
				return err
			}
			q := c.client.From("work_items").
				Select(workItemListSelect, "", false).
				Eq("workspace_id", ws).
				Is("deleted_at", "null")
			if archived {
				q = q.Not("archived_at", "is", "null")
			} else {
				q = q.Is("archived_at", "null")
			}
			if kind != "" {
				q = q.Eq("kind", kind)
			}
			raw, _, err := q.
				Order("updated_at", &postgrest.OrderOpts{Ascending: false}).
				Limit(lf.limit, "").
				Execute()
			if err != nil {
				return err
			}
			return printList(raw, workItemCols, lf.jsonOut)
		},
	}
	addListFlags(cmd, &lf)
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by kind (task, issue, assessment, review, ...)")
	cmd.Flags().BoolVar(&archived, "archived", false, "Show archived items instead of active ones")
	return cmd
}

func workItemsGetCmd(rf *auth.RootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id|ref>",
		Short: "Show one work item as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := connect(cmd.Context(), rf)
			if err != nil {
				return err
			}
			ws, err := c.requireWorkspace()
			if err != nil {
				return err
			}
			raw, _, err := workItemQuery(c, ws, args[0], workItemGetSelect).Execute()
			if err != nil {
				return err
			}
			return printJSON(raw)
		},
	}
}

// workItemQuery builds a single-row lookup by id or ref.
func workItemQuery(c *conn, ws, arg, selectCols string) *postgrest.FilterBuilder {
	col := "id"
	if !isUUID(arg) {
		col = "ref"
	}
	return c.client.From("work_items").
		Select(selectCols, "", false).
		Eq("workspace_id", ws).
		Is("deleted_at", "null").
		Eq(col, arg).
		Single()
}

func workItemsUpdateCmd(rf *auth.RootFlags) *cobra.Command {
	var (
		status  string
		due     string
		newName string
	)
	cmd := &cobra.Command{
		Use:   "update <id|ref>",
		Short: "Update a work item's status, due date, or name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if status == "" && due == "" && newName == "" {
				return errors.New("nothing to update — pass --status, --due, and/or --name")
			}
			c, err := connect(cmd.Context(), rf)
			if err != nil {
				return err
			}
			ws, err := c.requireWorkspace()
			if err != nil {
				return err
			}

			raw, _, err := workItemQuery(c, ws, args[0], "id,name,status_group_id").Execute()
			if err != nil {
				return fmt.Errorf("work item %q not found: %w", args[0], err)
			}
			var item struct {
				ID            string  `json:"id"`
				Name          string  `json:"name"`
				StatusGroupID *string `json:"status_group_id"`
			}
			if err := json.Unmarshal(raw, &item); err != nil {
				return fmt.Errorf("decode work item: %w", err)
			}

			patch := map[string]any{}
			if newName != "" {
				patch["name"] = newName
			}
			if due != "" {
				t, err := parseDue(due)
				if err != nil {
					return err
				}
				patch["due_at"] = t.Format(time.RFC3339)
			}
			if status != "" {
				// The DB constraint work_items_status_group_matches_status
				// requires status_id and status_group_id to agree, so both
				// columns are patched together (mirrors the web app).
				sid, gid, err := resolveStatus(c, item.StatusGroupID, status)
				if err != nil {
					return err
				}
				patch["status_id"] = sid
				patch["status_group_id"] = gid
			}

			updated, _, err := c.client.From("work_items").
				Update(patch, "representation", "").
				Eq("id", item.ID).
				Execute()
			if err != nil {
				return err
			}
			var rows []map[string]any
			if err := json.Unmarshal(updated, &rows); err == nil && len(rows) == 0 {
				return fmt.Errorf("no row updated — check your permissions in this workspace")
			}
			fmt.Fprintf(os.Stderr, "Updated %s (%s)\n", item.Name, item.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "New status by name (resolved within the item's status group)")
	cmd.Flags().StringVar(&due, "due", "", "New due date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&newName, "name", "", "New name")
	return cmd
}

func workItemsArchiveCmd(rf *auth.RootFlags, archive bool) *cobra.Command {
	use, short := "archive <id|ref>", "Archive a work item"
	if !archive {
		use, short = "restore <id|ref>", "Restore an archived work item"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := connect(cmd.Context(), rf)
			if err != nil {
				return err
			}
			ws, err := c.requireWorkspace()
			if err != nil {
				return err
			}
			raw, _, err := workItemQuery(c, ws, args[0], "id,name").Execute()
			if err != nil {
				return fmt.Errorf("work item %q not found: %w", args[0], err)
			}
			var item struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(raw, &item); err != nil {
				return fmt.Errorf("decode work item: %w", err)
			}
			patch := map[string]any{"archived_at": nil}
			verb := "Restored"
			if archive {
				patch["archived_at"] = time.Now().UTC().Format(time.RFC3339)
				verb = "Archived"
			}
			if _, _, err := c.client.From("work_items").
				Update(patch, "minimal", "").
				Eq("id", item.ID).
				Execute(); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "%s %s (%s)\n", verb, item.Name, item.ID)
			return nil
		},
	}
}

// resolveStatus finds a status by (case-insensitive) name, pinned to the
// item's status group when it has one — matching the web app's behavior and
// avoiding cross-group name collisions.
func resolveStatus(c *conn, statusGroupID *string, name string) (statusID, groupID string, err error) {
	q := c.client.From("statuses").Select("id,name,category,status_group_id", "", false)
	if statusGroupID != nil && *statusGroupID != "" {
		q = q.Eq("status_group_id", *statusGroupID)
	}
	raw, _, err := q.Execute()
	if err != nil {
		return "", "", err
	}
	var statuses []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Category      string `json:"category"`
		StatusGroupID string `json:"status_group_id"`
	}
	if err := json.Unmarshal(raw, &statuses); err != nil {
		return "", "", fmt.Errorf("decode statuses: %w", err)
	}

	var matches []int
	names := make([]string, 0, len(statuses))
	for i, s := range statuses {
		names = append(names, s.Name)
		if strings.EqualFold(s.Name, name) {
			matches = append(matches, i)
		}
	}
	switch len(matches) {
	case 0:
		return "", "", fmt.Errorf("no status named %q — available: %s", name, strings.Join(names, ", "))
	case 1:
		s := statuses[matches[0]]
		return s.ID, s.StatusGroupID, nil
	default:
		var opts []string
		for _, i := range matches {
			opts = append(opts, fmt.Sprintf("%s (%s, group %s)", statuses[i].Name, statuses[i].Category, statuses[i].StatusGroupID))
		}
		return "", "", fmt.Errorf("status name %q is ambiguous across status groups: %s", name, strings.Join(opts, "; "))
	}
}

// parseDue accepts a bare date or a full RFC3339 timestamp.
func parseDue(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --due %q (want YYYY-MM-DD or RFC3339)", s)
}
