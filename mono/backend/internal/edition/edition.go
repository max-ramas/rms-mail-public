package edition

import (
	"os"
	"strings"
)

// Edition represents the deployment mode of the RMS Mail application.
type Edition int

const (
	Mono    Edition = iota // Single-user, self-hosted local mail
	Unified                // Multi-account aggregator (default)
	Teams                  // Team collaboration with assignments
	MonoPro                // Commercial single-tenant local mail
)

const (
	ProductIDMono    = "502ef74b-4fbf-43e7-833d-f8efa62a2499"
	ProductIDUnified = "a8a17bc3-78f9-41b5-8f1f-1be874aa8c4d"
	ProductIDTeams   = "696e3a3a-440f-4759-a566-18a987ac011f"
	ProductIDMonoPro = "8d2f2b4c-98c4-4b9d-b63e-f6a8e8f2c691"
)

// String returns a human-readable name for the edition.
func (e Edition) String() string {
	switch e {
	case Mono:
		return "mono"
	case Unified:
		return "unified"
	case Teams:
		return "teams"
	case MonoPro:
		return "mono_pro"
	default:
		return "unknown"
	}
}

// ProductID returns the license server Product ID for the edition.
func (e Edition) ProductID() string {
	switch e {
	case Mono:
		return ProductIDMono
	case Unified:
		return ProductIDUnified
	case Teams:
		return ProductIDTeams
	case MonoPro:
		return ProductIDMonoPro
	default:
		return ProductIDUnified
	}
}

// Current is the active edition. Default is Unified.
var Current = Unified

// IsMono returns true when running in Mono edition.
func IsMono() bool { return Current == Mono }

// IsUnified returns true when running in Unified edition.
func IsUnified() bool { return Current == Unified }

// IsTeams returns true when running in Teams edition.
func IsTeams() bool { return Current == Teams }

// IsMonoPro returns true when running in MonoPro edition.
func IsMonoPro() bool { return Current == MonoPro }

// Init reads the EDITION environment variable and sets the Current edition.
// Valid values: "mono", "unified", "teams", "mono_pro". Default is "unified".
func Init() {
	switch strings.ToLower(os.Getenv("EDITION")) {
	case "mono":
		Current = Mono
	case "unified":
		Current = Unified
	case "teams":
		Current = Teams
	case "mono_pro", "monopro":
		Current = MonoPro
	default:
		Current = Unified
	}
}
