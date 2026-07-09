package inspection

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/gaie"
	"github.com/sonda-red/kleym/internal/spirecm"
)

func TestStatusHealthyInstallation(t *testing.T) {
	binding := testStatusBinding("binding-a", metav1.ConditionTrue)
	inspector := newTestStatusInspector(t, newReadyOperatorDeployment(), binding)

	report, err := inspector.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if report.Status != StatusResultOK ||
		report.Components.Operator.Status != StatusResultOK ||
		report.Components.SPIRECRDs.Status != StatusResultOK ||
		report.Summary.Bindings.OK != 1 ||
		report.Summary.Bindings.Error != 0 ||
		report.Summary.Bindings.Total != 1 ||
		report.Summary.Bindings.Conditions.Ready != 1 {
		t.Fatalf("unexpected healthy report: %#v", report)
	}
	if report.Components.Operator.Deployment != "kleym-system/kleym-operator" ||
		report.Components.Operator.ReadyReplicas != 1 ||
		report.Components.Operator.Replicas != 1 ||
		report.Components.Operator.Version != "v0.3.0" {
		t.Fatalf("operator = %#v, want deployment, ready replicas, and version", report.Components.Operator)
	}
	if report.Config.TrustDomain != "example.org" ||
		report.Config.ClusterSPIFFEIDClassName != "kleym" ||
		!report.Config.ClusterSPIFFEIDClassNameKnown {
		t.Fatalf("config = %#v, want operator config", report.Config)
	}
	if report.Components.GAIECRDs.InferencePool != "v1" {
		t.Fatalf("gaie = %#v, want served versions", report.Components.GAIECRDs)
	}
}

func TestStatusMissingSPIRECRD(t *testing.T) {
	binding := testStatusBinding("binding-a", metav1.ConditionFalse)
	binding.Status.Conditions[0].Reason = "ClusterSPIFFEIDCRDMissing"
	binding.Status.Conditions[0].Message = "ClusterSPIFFEID CRD is not installed"
	noSPIRE := removeStatusGVK(defaultStatusGVKs(), spirecm.ClusterSPIFFEIDGVK())
	inspector := newTestStatusInspectorWithGVKs(t, noSPIRE, newReadyOperatorDeployment(), binding)

	report, err := inspector.Status(context.Background())
	if !errors.Is(err, ErrStatusReportErrorFindings) {
		t.Fatalf("Status error = %v, want error findings", err)
	}

	if report.Components.SPIRECRDs.Status != StatusResultError ||
		report.Components.SPIRECRDs.Message != "missing ClusterSPIFFEID" {
		t.Fatalf("spire component = %#v, want missing ClusterSPIFFEID", report.Components.SPIRECRDs)
	}
	assertFinding(t, report.Findings, findingCRDMissing, BindingInspectionFindingSeverityError, "ClusterSPIFFEIDCRDMissing")
	if report.Summary.Bindings.Error != 1 {
		t.Fatalf("binding errors = %d, want 1", report.Summary.Bindings.Error)
	}
}

func TestStatusNoBindings(t *testing.T) {
	inspector := newTestStatusInspector(t, newReadyOperatorDeployment())

	report, err := inspector.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if report.Status != StatusResultOK ||
		report.Summary.Bindings.OK != 0 ||
		report.Summary.Bindings.Total != 0 {
		t.Fatalf("unexpected no-bindings report: %#v", report)
	}
	if report.Config.TrustDomain != "example.org" ||
		report.Config.ClusterSPIFFEIDClassName != "kleym" ||
		!report.Config.ClusterSPIFFEIDClassNameKnown {
		t.Fatalf("config = %#v, want operator args fallback", report.Config)
	}
}

func TestStatusBindingConfigFallbackReportsMixedValues(t *testing.T) {
	bindingA := testStatusBinding("binding-a", metav1.ConditionTrue)
	bindingB := testStatusBinding("binding-b", metav1.ConditionTrue)
	bindingB.Status.TrustDomain = "other.example.org"
	bindingB.Status.ClusterSPIFFEIDClassName = "other"
	operator := newReadyOperatorDeployment()
	operator.Spec.Template.Spec.Containers[0].Args = []string{}
	inspector := newTestStatusInspector(t, operator, bindingA, bindingB)

	report, err := inspector.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if report.Config.TrustDomain != "mixed" ||
		report.Config.ClusterSPIFFEIDClassName != "mixed" ||
		!report.Config.ClusterSPIFFEIDClassNameKnown {
		t.Fatalf("config = %#v, want mixed binding fallback", report.Config)
	}
}

func testStatusBinding(
	name string,
	readyStatus metav1.ConditionStatus,
) *kleymv1alpha1.InferenceIdentityBinding {
	binding := testInspectionBinding()
	binding.Name = name
	binding.Status.TrustDomain = "example.org"
	binding.Status.ClusterSPIFFEIDClassName = "kleym"
	binding.Status.Conditions = []metav1.Condition{{
		Type:               conditionTypeReady,
		Status:             readyStatus,
		Reason:             "Reconciled",
		Message:            "Binding reconciled",
		LastTransitionTime: metav1.Now(),
	}}
	binding.Status.RenderedSelectors = []kleymv1alpha1.RenderedSelectorStatus{{
		SpiffeID: "spiffe://kleym.sonda.red/ns/tenant-a/sa/model-sa/inference/pool/pool-a",
		Selectors: []string{
			"k8s:ns:tenant-a",
			"k8s:sa:model-sa",
			"k8s:pod-label:app:model-server",
		},
	}}
	return binding
}

func newReadyOperatorDeployment() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kleym-system",
			Name:      "kleym-operator",
			Labels: map[string]string{
				"app.kubernetes.io/name":      operatorLabelName,
				"app.kubernetes.io/component": operatorLabelComponent,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  operatorLabelComponent,
						Image: "ghcr.io/sonda-red/kleym-operator:v0.3.0",
						Args: []string{
							"--trust-domain=example.org",
							"--clusterspiffeid-class-name=kleym",
						},
					}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
}

func newTestStatusInspector(t *testing.T, objects ...client.Object) *statusInspector {
	return newTestStatusInspectorWithGVKs(t, defaultStatusGVKs(), objects...)
}

func newTestStatusInspectorWithGVKs(
	t *testing.T,
	available []schema.GroupVersionKind,
	objects ...client.Object,
) *statusInspector {
	t.Helper()

	scheme := newBindingInspectionScheme()
	_ = appsv1.AddToScheme(scheme)
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
	return &statusInspector{
		client: baseClient,
		mapper: newInspectionTestRESTMapper(
			defaultStatusGVKs(),
			available,
		),
		now: func() time.Time {
			return time.Date(2026, 5, 18, 10, 11, 12, 0, time.UTC)
		},
	}
}

func defaultStatusGVKs() []schema.GroupVersionKind {
	return append(
		[]schema.GroupVersionKind{
			kleymv1alpha1.GroupVersion.WithKind("InferenceIdentityBinding"),
			spirecm.ClusterSPIFFEIDGVK(),
		},
		gaie.InferencePoolGVKs()...,
	)
}

func removeStatusGVK(gvks []schema.GroupVersionKind, removed schema.GroupVersionKind) []schema.GroupVersionKind {
	filtered := make([]schema.GroupVersionKind, 0, len(gvks))
	for _, gvk := range gvks {
		if gvk != removed {
			filtered = append(filtered, gvk)
		}
	}
	return filtered
}

var _ StatusInspector = &statusInspector{}
