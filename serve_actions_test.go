package mirastack

import "testing"

func TestActionsToProto_EmitsRoutingSemantics(t *testing.T) {
	actions := []Action{{
		ID:          "query_instant",
		Description: "Run metric query",
		Permission:  PermissionRead,
		Stages:      []DevOpsStage{StageObserve},
		Routing: RoutingSemantics{
			SchemaVersion:         RoutingSemanticsSchemaVersionV1,
			AcceptedIntentDomains: []string{"core.observability.metric_query"},
			CapabilityDomain:      "core.observability.metric_query",
			PositiveUseCases:      []string{"Run metric query"},
			NegativeUseCases:      []string{"Trigger backups"},
			SignalDomains:         []string{"signal.metrics"},
			BackendDomains:        []string{"backend.victoriametrics"},
			EntityTypes:           []string{"entity.service"},
		},
	}}

	defs := actionsToProto(actions)
	if len(defs) != 1 {
		t.Fatalf("expected one action def, got %d", len(defs))
	}
	if defs[0].Routing.SchemaVersion != RoutingSemanticsSchemaVersionV1 {
		t.Fatalf("expected schema version %q, got %q", RoutingSemanticsSchemaVersionV1, defs[0].Routing.SchemaVersion)
	}
	if len(defs[0].Routing.AcceptedIntentDomains) != 1 || defs[0].Routing.AcceptedIntentDomains[0] != "core.observability.metric_query" {
		t.Fatalf("unexpected accepted domains: %#v", defs[0].Routing.AcceptedIntentDomains)
	}
}
