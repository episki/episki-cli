package resources

import (
	"context"
	"errors"
	"regexp"

	"github.com/episki/episki-cli/internal/api"
	"github.com/episki/episki-cli/internal/auth"
	"github.com/episki/episki-cli/internal/config"
	"github.com/supabase-community/postgrest-go"
)

// conn bundles the PostgREST client with the resolved credential so commands
// can read the workspace claim off the JWT.
type conn struct {
	client *postgrest.Client
	cred   auth.Credential
	cfg    config.Config
}

func connect(ctx context.Context, rf *auth.RootFlags) (*conn, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	client, cred, err := api.New(ctx, rf)
	if err != nil {
		return nil, err
	}
	return &conn{client: client, cred: cred, cfg: cfg}, nil
}

// workspaceID returns the workspace claim on the active token, or "".
func (c *conn) workspaceID() string { return auth.WorkspaceID(c.cred.Token) }

// errNoWorkspace explains the empty-claim state instead of letting RLS
// silently return zero rows, which reads as "no data" to a new user.
var errNoWorkspace = errors.New(
	"no workspace selected — run `episki workspaces use <id|slug>`\n" +
		"(or switch workspaces at the web app, then `episki auth refresh`)")

// requireWorkspace returns the JWT workspace claim, erroring when absent.
// Workspace-scoped RLS guarantees every entity read is empty without it.
func (c *conn) requireWorkspace() (string, error) {
	ws := c.workspaceID()
	if ws == "" {
		return "", errNoWorkspace
	}
	return ws, nil
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isUUID distinguishes ids from refs/slugs in `get <id|ref>` arguments.
func isUUID(s string) bool { return uuidRe.MatchString(s) }
