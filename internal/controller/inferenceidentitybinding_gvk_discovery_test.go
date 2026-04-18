package controller

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestFilterAvailableGVKs_AllAvailable(t *testing.T) {
	t.Parallel()

	mapper := newTestRESTMapper(inferenceObjectiveGVKs, inferenceObjectiveGVKs)
	available, err := filterAvailableGVKs(mapper, inferenceObjectiveGVKs, logr.Discard())
	if err != nil {
		t.Fatalf("filterAvailableGVKs returned error: %v", err)
	}

	if !equalGVKSets(available, inferenceObjectiveGVKs) {
		t.Fatalf("available GVKs = %v, want %v", available, inferenceObjectiveGVKs)
	}
}

func TestFilterAvailableGVKs_XOnlyAvailable(t *testing.T) {
	t.Parallel()

	mapper := newTestRESTMapper(inferenceObjectiveGVKs, []schema.GroupVersionKind{inferenceObjectiveGVKs[0]})
	available, err := filterAvailableGVKs(mapper, inferenceObjectiveGVKs, logr.Discard())
	if err != nil {
		t.Fatalf("filterAvailableGVKs returned error: %v", err)
	}

	want := []schema.GroupVersionKind{inferenceObjectiveGVKs[0]}
	if !equalGVKSets(available, want) {
		t.Fatalf("available GVKs = %v, want %v", available, want)
	}
}

func TestFilterAvailableGVKs_K8sOnlyAvailable(t *testing.T) {
	t.Parallel()

	mapper := newTestRESTMapper(inferenceObjectiveGVKs, []schema.GroupVersionKind{inferenceObjectiveGVKs[1]})
	available, err := filterAvailableGVKs(mapper, inferenceObjectiveGVKs, logr.Discard())
	if err != nil {
		t.Fatalf("filterAvailableGVKs returned error: %v", err)
	}

	want := []schema.GroupVersionKind{inferenceObjectiveGVKs[1]}
	if !equalGVKSets(available, want) {
		t.Fatalf("available GVKs = %v, want %v", available, want)
	}
}

func TestFilterAvailableGVKs_NoneAvailable(t *testing.T) {
	t.Parallel()

	mapper := newTestRESTMapper(inferenceObjectiveGVKs, nil)
	available, err := filterAvailableGVKs(mapper, inferenceObjectiveGVKs, logr.Discard())
	if err != nil {
		t.Fatalf("filterAvailableGVKs returned error: %v", err)
	}
	if len(available) != 0 {
		t.Fatalf("available GVKs = %v, want empty", available)
	}
}

func TestFilterAvailableGVKs_PropagatesUnexpectedRESTMapperError(t *testing.T) {
	t.Parallel()

	base := newTestRESTMapper(inferenceObjectiveGVKs, []schema.GroupVersionKind{inferenceObjectiveGVKs[0]})
	mapper := restMapperWithInjectedError{
		RESTMapper: base,
		groupKind:  inferenceObjectiveGVKs[0].GroupKind(),
		err:        errors.New("boom"),
	}

	_, err := filterAvailableGVKs(mapper, inferenceObjectiveGVKs, logr.Discard())
	if err == nil {
		t.Fatalf("filterAvailableGVKs returned nil error")
	}
	if !strings.Contains(err.Error(), "resolve REST mapping for") {
		t.Fatalf("error = %q, want context about REST mapping", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %q, want wrapped mapper error", err)
	}
}

type restMapperWithInjectedError struct {
	meta.RESTMapper
	groupKind schema.GroupKind
	err       error
}

func (m restMapperWithInjectedError) RESTMapping(
	groupKind schema.GroupKind,
	versions ...string,
) (*meta.RESTMapping, error) {
	if groupKind == m.groupKind {
		return nil, m.err
	}
	return m.RESTMapper.RESTMapping(groupKind, versions...)
}

func newTestRESTMapper(candidates []schema.GroupVersionKind, available []schema.GroupVersionKind) meta.RESTMapper {
	versions := uniqueGroupVersions(candidates)
	mapper := meta.NewDefaultRESTMapper(versions)
	for _, gvk := range available {
		mapper.Add(gvk, meta.RESTScopeNamespace)
	}
	return mapper
}

func uniqueGroupVersions(gvks []schema.GroupVersionKind) []schema.GroupVersion {
	seen := map[schema.GroupVersion]struct{}{}
	versions := make([]schema.GroupVersion, 0, len(gvks))
	for _, gvk := range gvks {
		gv := gvk.GroupVersion()
		if _, exists := seen[gv]; exists {
			continue
		}
		seen[gv] = struct{}{}
		versions = append(versions, gv)
	}
	return versions
}

func equalGVKSets(left []schema.GroupVersionKind, right []schema.GroupVersionKind) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
