package mirastack

import (
	"strings"

	"github.com/google/uuid"
)

// nameSpaceTenant is the fixed UUID5 namespace under which every MIRASTACK
// tenant identifier is derived. This value MUST match the engine's
// internal/tenants/id.go namespace exactly — they are both frozen to the
// same constant so the engine, SDKs, and miractl all derive the identical
// UUID5 from the same slug, deterministically and forever.
//
// The namespace was generated once via UUID5 of the URL
//
//	https://mirastack.ai/ns/tenants/v1
//
// under uuid.NameSpaceURL and is frozen here.
var nameSpaceTenant = uuid.MustParse("f9f3a4d4-2c64-5b9e-9e25-8a8b6f6f6f6f")

// IDFromSlug derives the canonical UUID5 tenant identifier for the given
// slug. The input is normalised (lowercased, trimmed) before hashing.
// The returned string is the 36-character hex representation
// (e.g. "f9f3a4d4-2c64-5b9e-9e25-...").
//
// IDFromSlug is deterministic: the same slug always yields the same UUID5.
// This is the property that lets plugin operators resolve their tenant ID
// without a live engine connection.
func IDFromSlug(slug string) string {
	return uuid.NewSHA1(nameSpaceTenant, []byte("tenant:"+strings.ToLower(strings.TrimSpace(slug)))).String()
}
