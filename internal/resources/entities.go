package resources

import (
	"github.com/episki/episki-cli/internal/auth"
	"github.com/spf13/cobra"
)

// Column shapes below are pinned to the base repo's generated types
// (core/app/types/database-generated.types.ts, public schema). Heavy JSON
// columns (TipTap description/content, payload) are deliberately excluded.
//
// The kinds covered here are a subset of core's entity registry
// (server/utils/entityRegistry.ts), which the app publishes at
// GET /api/ontology/catalog for exactly this reason — out-of-process
// consumers otherwise hand-maintain a copy that goes stale in silence. When
// adding a kind, take its table and ref column from there rather than
// guessing.

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

func vendorsCmd(rf *auth.RootFlags) *cobra.Command {
	// The vendor record absorbed subprocessors in core #346, so this is the
	// single third-party row: tier and status drive the review cycle, and
	// next_review_at is maintained by a trigger, not by clients.
	return resourceSpec{
		use:        "vendors",
		short:      "Third-party vendors in the workspace",
		table:      "vendors",
		listSelect: "id,name,tier,status,data_classification,next_review_at,updated_at,vendor_categories(name)",
		getSelect: "id,name,tier,status,purpose,data_classification,regions,mfa_required,sso_enabled," +
			"annual_spend_usd,contract_start_date,contract_end_date,review_frequency_months," +
			"last_reviewed_at,next_review_at,last_monitored_at,owner_id,responsible_owner_id," +
			"created_at,updated_at,vendor_categories(name)",
		cols: []column{
			{"id", "id"}, {"name", "name"}, {"tier", "tier"}, {"status", "status"},
			{"category", "vendor_categories.name"}, {"data", "data_classification"},
			{"next review", "next_review_at"}, {"updated", "updated_at"},
		},
	}.command(rf)
}

func programsCmd(rf *auth.RootFlags) *cobra.Command {
	return resourceSpec{
		use:        "programs",
		short:      "Compliance programs in the workspace",
		table:      "programs",
		listSelect: "id,name,status,created_at,updated_at",
		getSelect:  "id,name,status,owner_id,manager_id,created_at,updated_at",
		cols: []column{
			{"id", "id"}, {"name", "name"}, {"status", "status"},
			{"created", "created_at"}, {"updated", "updated_at"},
		},
	}.command(rf)
}

func obligationsCmd(rf *auth.RootFlags) *cobra.Command {
	return resourceSpec{
		use:        "obligations",
		short:      "Obligations in the workspace",
		table:      "obligations",
		listSelect: "id,ref,name,source_kind,created_at,updated_at",
		getSelect:  "id,ref,name,source_kind,owner_id,created_at,updated_at",
		refCol:     "ref",
		cols: []column{
			{"id", "id"}, {"ref", "ref"}, {"name", "name"},
			{"source", "source_kind"}, {"updated", "updated_at"},
		},
	}.command(rf)
}

func exceptionsCmd(rf *auth.RootFlags) *cobra.Command {
	// An approved, in-window exception can satisfy an assertion (core #328),
	// so the window columns are the ones worth seeing at a glance.
	return resourceSpec{
		use:        "exceptions",
		short:      "Control exceptions in the workspace",
		table:      "exceptions",
		listSelect: "id,name,status,assertion_slug,starts_at,ends_at,updated_at",
		getSelect: "id,name,status,assertion_slug,match_field,starts_at,ends_at," +
			"approved_at,approver_id,owner_id,created_at,updated_at",
		cols: []column{
			{"id", "id"}, {"name", "name"}, {"status", "status"},
			{"assertion", "assertion_slug"}, {"starts", "starts_at"},
			{"ends", "ends_at"}, {"updated", "updated_at"},
		},
	}.command(rf)
}

func goalsCmd(rf *auth.RootFlags) *cobra.Command {
	return resourceSpec{
		use:        "goals",
		short:      "Goals in the workspace",
		table:      "goals",
		listSelect: "id,name,status,due_date,updated_at",
		getSelect:  "id,name,status,due_date,owner_id,created_at,updated_at",
		cols: []column{
			{"id", "id"}, {"name", "name"}, {"status", "status"},
			{"due", "due_date"}, {"updated", "updated_at"},
		},
	}.command(rf)
}
