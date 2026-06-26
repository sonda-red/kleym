package controller

import "testing"

func TestOperatorConfigWithEnvFallbackUsesEnvWhenFlagsOmitted(t *testing.T) {
	t.Parallel()

	config := (OperatorConfig{}).WithEnvFallback(
		mapLookupEnv(map[string]string{
			EnvTrustDomain:                 "example.org",
			EnvClusterSPIFFEIDClassName:    "kleym",
			"UNRELATED_OPERATOR_ENV_VALUE": "ignored",
		}),
		OperatorConfigExplicitFields{},
	)

	if config.TrustDomain != "example.org" {
		t.Fatalf("TrustDomain = %q, want %q", config.TrustDomain, "example.org")
	}
	if config.ClusterSPIFFEIDClassName != "kleym" {
		t.Fatalf("ClusterSPIFFEIDClassName = %q, want %q", config.ClusterSPIFFEIDClassName, "kleym")
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestOperatorConfigWithEnvFallbackKeepsExplicitFlags(t *testing.T) {
	t.Parallel()

	config := (OperatorConfig{
		TrustDomain:              "flag.example.org",
		ClusterSPIFFEIDClassName: "flag-class",
	}).WithEnvFallback(
		mapLookupEnv(map[string]string{
			EnvTrustDomain:              "env.example.org",
			EnvClusterSPIFFEIDClassName: "env-class",
		}),
		OperatorConfigExplicitFields{
			TrustDomain:              true,
			ClusterSPIFFEIDClassName: true,
		},
	)

	if config.TrustDomain != "flag.example.org" {
		t.Fatalf("TrustDomain = %q, want %q", config.TrustDomain, "flag.example.org")
	}
	if config.ClusterSPIFFEIDClassName != "flag-class" {
		t.Fatalf("ClusterSPIFFEIDClassName = %q, want %q", config.ClusterSPIFFEIDClassName, "flag-class")
	}
}

func TestOperatorConfigWithEnvFallbackKeepsExplicitEmptyClassName(t *testing.T) {
	t.Parallel()

	config := (OperatorConfig{
		TrustDomain: "example.org",
	}).WithEnvFallback(
		mapLookupEnv(map[string]string{
			EnvClusterSPIFFEIDClassName: "env-class",
		}),
		OperatorConfigExplicitFields{
			ClusterSPIFFEIDClassName: true,
		},
	)

	if config.ClusterSPIFFEIDClassName != "" {
		t.Fatalf("ClusterSPIFFEIDClassName = %q, want empty", config.ClusterSPIFFEIDClassName)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestOperatorConfigWithEnvFallbackKeepsExplicitEmptyTrustDomain(t *testing.T) {
	t.Parallel()

	config := (OperatorConfig{}).WithEnvFallback(
		mapLookupEnv(map[string]string{
			EnvTrustDomain: "env.example.org",
		}),
		OperatorConfigExplicitFields{
			TrustDomain: true,
		},
	)

	err := config.Validate()
	if err == nil {
		t.Fatalf("expected missing trustDomain error, got nil")
	}
	if err.Error() != trustDomainRequiredMessage {
		t.Fatalf("error = %q, want %q", err.Error(), trustDomainRequiredMessage)
	}
}

func TestOperatorConfigWithEnvFallbackValidatesEnvValues(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]string{
		"invalid-trust-domain": {
			EnvTrustDomain: "spiffe://example.org",
		},
		"invalid-class-name": {
			EnvTrustDomain:              "example.org",
			EnvClusterSPIFFEIDClassName: " kleym",
		},
	}

	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			config := (OperatorConfig{}).WithEnvFallback(mapLookupEnv(env), OperatorConfigExplicitFields{})
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error, got nil")
			}
		})
	}
}

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

func mapLookupEnv(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
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
