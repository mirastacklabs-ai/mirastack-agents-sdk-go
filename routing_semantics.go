package mirastack

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	RoutingSemanticsSchemaVersionV1 = "mirastack.routing_semantics/v1"

	maxRoutingDomainLength   = 128
	maxRoutingUseCaseLength  = 256
	maxRoutingDomainListSize = 64
	maxRoutingUseCaseList    = 64
)

var routingDomainRE = regexp.MustCompile(`^[a-z][a-z0-9_-]*(?:\.[a-z][a-z0-9_-]*)+$`)

type RoutingSemantics struct {
	SchemaVersion         string
	AcceptedIntentDomains []string
	CapabilityDomain      string
	PositiveUseCases      []string
	NegativeUseCases      []string
	SignalDomains         []string
	BackendDomains        []string
	EntityTypes           []string
}

func normalizeDomainList(values []string, field string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > maxRoutingDomainListSize {
		return nil, fmt.Errorf("%s exceeds max items %d", field, maxRoutingDomainListSize)
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if len(normalized) > maxRoutingDomainLength {
			return nil, fmt.Errorf("%s item %q exceeds max length %d", field, normalized, maxRoutingDomainLength)
		}
		if !routingDomainRE.MatchString(normalized) {
			return nil, fmt.Errorf("%s item %q is not namespaced", field, normalized)
		}
		if _, exists := seen[normalized]; exists {
			return nil, fmt.Errorf("%s contains duplicate domain %q", field, normalized)
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil, nil
	}
	slices.Sort(out)
	return out, nil
}

func normalizeUseCaseList(values []string, field string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > maxRoutingUseCaseList {
		return nil, fmt.Errorf("%s exceeds max items %d", field, maxRoutingUseCaseList)
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if len(normalized) > maxRoutingUseCaseLength {
			return nil, fmt.Errorf("%s item %q exceeds max length %d", field, normalized, maxRoutingUseCaseLength)
		}
		if _, exists := seen[normalized]; exists {
			return nil, fmt.Errorf("%s contains duplicate use case %q", field, normalized)
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil, nil
	}
	slices.Sort(out)
	return out, nil
}

func normalizeSingleDomain(value, field string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", nil
	}
	if len(normalized) > maxRoutingDomainLength {
		return "", fmt.Errorf("%s %q exceeds max length %d", field, normalized, maxRoutingDomainLength)
	}
	if !routingDomainRE.MatchString(normalized) {
		return "", fmt.Errorf("%s %q is not namespaced", field, normalized)
	}
	return normalized, nil
}

func (r RoutingSemantics) NormalizeAndValidate() (RoutingSemantics, error) {
	out := r
	out.SchemaVersion = strings.TrimSpace(out.SchemaVersion)
	if out.SchemaVersion == "" {
		return RoutingSemantics{}, fmt.Errorf("routing schema_version is required")
	}
	if out.SchemaVersion != RoutingSemanticsSchemaVersionV1 {
		return RoutingSemantics{}, fmt.Errorf("unsupported routing schema_version %q", out.SchemaVersion)
	}

	accepted, err := normalizeDomainList(out.AcceptedIntentDomains, "accepted_intent_domains")
	if err != nil {
		return RoutingSemantics{}, err
	}
	if len(accepted) == 0 {
		return RoutingSemantics{}, fmt.Errorf("accepted_intent_domains must contain at least one domain")
	}
	out.AcceptedIntentDomains = accepted

	capability, err := normalizeSingleDomain(out.CapabilityDomain, "capability_domain")
	if err != nil {
		return RoutingSemantics{}, err
	}
	if capability == "" {
		return RoutingSemantics{}, fmt.Errorf("capability_domain is required")
	}
	out.CapabilityDomain = capability

	positive, err := normalizeUseCaseList(out.PositiveUseCases, "positive_use_cases")
	if err != nil {
		return RoutingSemantics{}, err
	}
	if len(positive) == 0 {
		return RoutingSemantics{}, fmt.Errorf("positive_use_cases must contain at least one use case")
	}
	out.PositiveUseCases = positive

	negative, err := normalizeUseCaseList(out.NegativeUseCases, "negative_use_cases")
	if err != nil {
		return RoutingSemantics{}, err
	}
	out.NegativeUseCases = negative

	signalDomains, err := normalizeDomainList(out.SignalDomains, "signal_domains")
	if err != nil {
		return RoutingSemantics{}, err
	}
	out.SignalDomains = signalDomains

	backendDomains, err := normalizeDomainList(out.BackendDomains, "backend_domains")
	if err != nil {
		return RoutingSemantics{}, err
	}
	out.BackendDomains = backendDomains

	entityTypes, err := normalizeDomainList(out.EntityTypes, "entity_types")
	if err != nil {
		return RoutingSemantics{}, err
	}
	out.EntityTypes = entityTypes

	return out, nil
}
