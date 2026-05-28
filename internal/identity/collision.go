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
package identity

import (
	"encoding/json"
	"fmt"
)

// PerObjectiveCollisionFingerprint produces the deterministic key used for collision checks.
func PerObjectiveCollisionFingerprint(
	identity RenderedIdentity,
	containerName string,
) (string, error) {
	if _, err := SelectorForContainerName(containerName); err != nil {
		return "", fmt.Errorf("containerName is invalid for per-objective collision detection: %w", err)
	}

	podSelectorFingerprint, err := normalizedPodSelectorFingerprint(identity.PodSelector)
	if err != nil {
		return "", err
	}

	selectorFingerprint, err := normalizedSelectorFingerprint(identity.Selectors)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s|%s|%s", podSelectorFingerprint, selectorFingerprint, containerName), nil
}

func normalizedPodSelectorFingerprint(selector map[string]any) (string, error) {
	if len(selector) == 0 {
		return "", fmt.Errorf("pod selector must be present for collision detection")
	}

	serialized, err := json.Marshal(selector)
	if err != nil {
		return "", fmt.Errorf("failed to encode pod selector fingerprint: %w", err)
	}

	return string(serialized), nil
}

func normalizedSelectorFingerprint(selectors []string) (string, error) {
	if len(selectors) == 0 {
		return "", fmt.Errorf("selectors must be present for collision detection")
	}

	serialized, err := json.Marshal(selectors)
	if err != nil {
		return "", fmt.Errorf("failed to encode selector fingerprint: %w", err)
	}

	return string(serialized), nil
}
