package resources

import (
	"github.com/episki/episki-cli/internal/auth"
	"github.com/spf13/cobra"
)

// Column shapes below are pinned to the base repo's generated types
// (core/app/types/database-generated.types.ts, public schema). Heavy JSON
// columns (TipTap description/content, payload) are deliberately excluded.

func frameworksCmd(rf *auth.RootFlags) *cobra.Command {
	return resourceSpec{
		use:        "frameworks",
		short:      "Compliance frameworks in the workspace",
		table:      "frameworks",
		listSelect: "id,name,version,source,created_at,updated_at",
		getSelect:  "id,name,version,source,owner_id,created_at,updated_at",
		cols: []column{
			{"id", "id"}, {"name", "name"}, {"version", "version"},
			{"source", "source"}, {"updated", "updated_at"},
		},
	}.command(rf)
}

func controlsCmd(rf *auth.RootFlags) *cobra.Command {
	// Framework filtering is a follow-up: public-schema controls link to
	// frameworks through the generic `relationships` ontology table, not a
	// dedicated join table, so it needs the edge-type conventions first.
	return resourceSpec{
		use:        "controls",
		short:      "Controls in the workspace",
		table:      "controls",
		listSelect: "id,ref,name,status,control_type,is_group,updated_at",
		getSelect:  "id,ref,name,status,control_type,implementation,is_group,parent_id,owner_id,subsets,created_at,updated_at",
		refCol:     "ref",
		cols: []column{
			{"id", "id"}, {"ref", "ref"}, {"name", "name"}, {"status", "status"},
			{"type", "control_type"}, {"group", "is_group"}, {"updated", "updated_at"},
		},
	}.command(rf)
}

func evidenceCmd(rf *auth.RootFlags) *cobra.Command {
	return resourceSpec{
		use:        "evidence",
		short:      "Evidence records in the workspace",
		table:      "evidence",
		listSelect: "id,name,evidence_type,source,collected_at,valid_until,updated_at",
		getSelect:  "id,name,evidence_type,source,collected_at,valid_until,owner_id,created_at,updated_at",
		cols: []column{
			{"id", "id"}, {"name", "name"}, {"type", "evidence_type"},
			{"source", "source"}, {"collected", "collected_at"},
			{"valid until", "valid_until"}, {"updated", "updated_at"},
		},
	}.command(rf)
}

func policiesCmd(rf *auth.RootFlags) *cobra.Command {
	return resourceSpec{
		use:        "policies",
		short:      "Policies in the workspace",
		table:      "policies",
		listSelect: "id,name,status,version,approved_at,updated_at",
		getSelect:  "id,name,status,version,approved_at,approved_by,owner_id,created_at,updated_at",
		cols: []column{
			{"id", "id"}, {"name", "name"}, {"status", "status"},
			{"version", "version"}, {"approved", "approved_at"}, {"updated", "updated_at"},
		},
	}.command(rf)
}

func risksCmd(rf *auth.RootFlags) *cobra.Command {
	return resourceSpec{
		use:        "risks",
		short:      "Risks in the workspace",
		table:      "risks",
		listSelect: "id,ref,name,status,treatment,residual_impact,residual_likelihood,updated_at",
		getSelect:  "id,ref,name,status,treatment,inherent_impact,inherent_likelihood,residual_impact,residual_likelihood,assessed_at,accepted_at,owner_id,created_at,updated_at",
		refCol:     "ref",
		cols: []column{
			{"id", "id"}, {"ref", "ref"}, {"name", "name"}, {"status", "status"},
			{"treatment", "treatment"}, {"impact", "residual_impact"},
			{"likelihood", "residual_likelihood"}, {"updated", "updated_at"},
		},
	}.command(rf)
}
