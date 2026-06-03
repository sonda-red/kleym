package controller

import "testing"

func TestOperatorConfigValidateRequiresTrustDomain(t *testing.T) {
	t.Parallel()

	err := (OperatorConfig{}).Validate()
	if err == nil {
		t.Fatalf("expected missing trustDomain error, got nil")
	}
	if err.Error() != trustDomainRequiredMessage {
		t.Fatalf("error = %q, want %q", err.Error(), trustDomainRequiredMessage)
	}
}

func TestOperatorConfigValidateRejectsAmbiguousValues(t *testing.T) {
	t.Parallel()

	cases := map[string]OperatorConfig{
		"trust-domain-leading-whitespace": {
			TrustDomain: " example.org",
		},
		"trust-domain-scheme": {
			TrustDomain: "spiffe://example.org",
		},
		"trust-domain-path": {
			TrustDomain: "example.org/ns",
		},
		"class-name-whitespace": {
			TrustDomain:              "example.org",
			ClusterSPIFFEIDClassName: " kleym",
		},
	}

	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error, got nil")
			}
		})
	}
}

func TestOperatorConfigValidateAcceptsOptionalClassName(t *testing.T) {
	t.Parallel()

	for name, config := range map[string]OperatorConfig{
		"classless": {TrustDomain: "example.org"},
		"classed": {
			TrustDomain:              "example.org",
			ClusterSPIFFEIDClassName: "kleym",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := config.Validate(); err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
		})
	}
}
