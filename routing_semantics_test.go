package mirastack

import "testing"

func TestRoutingSemanticsNormalizeAndValidate_Valid(t *testing.T) {
	in := RoutingSemantics{
		SchemaVersion:         RoutingSemanticsSchemaVersionV1,
		AcceptedIntentDomains: []string{" Core.Observability.Metric_Query "},
		CapabilityDomain:      "core.observability.metric_query",
		PositiveUseCases:      []string{"Run metric query"},
		NegativeUseCases:      []string{"Manage backup jobs"},
		SignalDomains:         []string{"signal.metrics"},
		BackendDomains:        []string{"backend.victoriametrics"},
		EntityTypes:           []string{"entity.service"},
	}
	out, err := in.NormalizeAndValidate()
	if err != nil {
		t.Fatalf("expected valid semantics, got error: %v", err)
	}
	if len(out.AcceptedIntentDomains) != 1 || out.AcceptedIntentDomains[0] != "core.observability.metric_query" {
		t.Fatalf("unexpected accepted domains: %#v", out.AcceptedIntentDomains)
	}
}

func TestRoutingSemanticsNormalizeAndValidate_RejectsMissingRequired(t *testing.T) {
	if _, err := (RoutingSemantics{
		SchemaVersion: RoutingSemanticsSchemaVersionV1,
	}).NormalizeAndValidate(); err == nil {
		t.Fatal("expected missing required fields to fail")
	}
}

func TestRoutingSemanticsNormalizeAndValidate_RejectsInvalidDomain(t *testing.T) {
	if _, err := (RoutingSemantics{
		SchemaVersion:         RoutingSemanticsSchemaVersionV1,
		AcceptedIntentDomains: []string{"invalid"},
		CapabilityDomain:      "core.observability.metric_query",
		PositiveUseCases:      []string{"Run metric query"},
	}).NormalizeAndValidate(); err == nil {
		t.Fatal("expected invalid domain to fail")
	}
}
