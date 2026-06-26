/*
Copyright 2026 Kalin Daskalov.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"fmt"
	"strings"
)

const trustDomainRequiredMessage = "trustDomain must be configured before Kleym can render SPIFFE IDs"

const (
	// EnvTrustDomain is the startup-only fallback for --trust-domain.
	EnvTrustDomain = "KLEYM_TRUST_DOMAIN"
	// EnvClusterSPIFFEIDClassName is the startup-only fallback for --clusterspiffeid-class-name.
	EnvClusterSPIFFEIDClassName = "KLEYM_CLUSTERSPIFFEID_CLASS_NAME"
)

// OperatorConfig carries install-level settings that affect all reconciled identities.
type OperatorConfig struct {
	TrustDomain              string
	ClusterSPIFFEIDClassName string
}

// OperatorConfigExplicitFields records startup flags that must override env fallbacks.
type OperatorConfigExplicitFields struct {
	TrustDomain              bool
	ClusterSPIFFEIDClassName bool
}

// WithEnvFallback returns config with env values filled only for omitted startup flags.
func (c OperatorConfig) WithEnvFallback(
	lookupEnv func(string) (string, bool),
	explicit OperatorConfigExplicitFields,
) OperatorConfig {
	if !explicit.TrustDomain {
		if value, ok := lookupEnv(EnvTrustDomain); ok {
			c.TrustDomain = value
		}
	}
	if !explicit.ClusterSPIFFEIDClassName {
		if value, ok := lookupEnv(EnvClusterSPIFFEIDClassName); ok {
			c.ClusterSPIFFEIDClassName = value
		}
	}
	return c
}

// Validate rejects ambiguous install-level identity configuration before reconciliation starts.
func (c OperatorConfig) Validate() error {
	if strings.TrimSpace(c.TrustDomain) == "" {
		return fmt.Errorf("%s", trustDomainRequiredMessage)
	}
	if c.TrustDomain != strings.TrimSpace(c.TrustDomain) {
		return fmt.Errorf("trustDomain must not include leading or trailing whitespace")
	}
	if strings.HasPrefix(c.TrustDomain, "spiffe://") {
		return fmt.Errorf("trustDomain must not include spiffe://")
	}
	if strings.Contains(c.TrustDomain, "/") {
		return fmt.Errorf("trustDomain must not contain /")
	}
	if c.ClusterSPIFFEIDClassName != strings.TrimSpace(c.ClusterSPIFFEIDClassName) {
		return fmt.Errorf("clusterspiffeidClassName must not include leading or trailing whitespace")
	}
	return nil
}
