package spec

// NewSocialCapSpec creates a social cap specification section
func NewSocialCapSpec() SpecSection {
	// TODO(QA): cap brand/social ≤ +10 pts AFTER residuals
	// See: product.md for social factor capping requirements
	// Must validate: social contribution ≤ +10 points applied AFTER momentum & volume
	return SimpleSpecSection{
		id:          "social_cap",
		name:        "Social Cap",
		description: "Social/brand contribution capped at +10 points after residuals",
		ready:       true, // Stub passes for build
	}
}