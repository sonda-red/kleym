package controller

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestSetupWithManagerStartsAndReconcilesWithXObjectiveOnlyCRD(t *testing.T) {
	t.Helper()

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := kleymv1alpha1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add kleym scheme: %v", err)
	}
	registerEnvtestUnstructuredGVK(testScheme, clusterSPIFFEIDGVK)
	for _, gvk := range inferenceObjectiveGVKs {
		registerEnvtestUnstructuredGVK(testScheme, gvk)
	}
	for _, gvk := range inferencePoolGVKs {
		registerEnvtestUnstructuredGVK(testScheme, gvk)
	}

	crdDir := t.TempDir()
	copyCRDFile(t, filepath.Join("testdata", "crds", "inference.networking.x-k8s.io_inferenceobjectives.yaml"), crdDir)
	copyCRDFile(t, filepath.Join("testdata", "crds", "inference.networking.x-k8s.io_inferencepools.yaml"), crdDir)
	copyCRDFile(t, filepath.Join("testdata", "crds", "inference.networking.k8s.io_inferencepools.yaml"), crdDir)
	copyCRDFile(t, filepath.Join("testdata", "crds", "spire.spiffe.io_clusterspiffeids.yaml"), crdDir)

	testEnvironment := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			crdDir,
		},
		ErrorIfCRDPathMissing: true,
	}
	if binDir := getFirstFoundEnvTestBinaryDir(); binDir != "" {
		testEnvironment.BinaryAssetsDirectory = binDir
	}

	cfg, err := testEnvironment.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	t.Cleanup(func() {
		if stopErr := testEnvironment.Stop(); stopErr != nil {
			t.Fatalf("stop envtest: %v", stopErr)
		}
	})

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 testScheme,
		Metrics:                server.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		Controller: config.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	reconciler := &InferenceIdentityBindingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		t.Fatalf("setup controller with partial CRDs: %v", err)
	}

	if len(reconciler.availableObjectiveGVKs) != 1 || reconciler.availableObjectiveGVKs[0] != inferenceObjectiveGVKs[0] {
		t.Fatalf("availableObjectiveGVKs = %v, want [%v]", reconciler.availableObjectiveGVKs, inferenceObjectiveGVKs[0])
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- mgr.Start(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case runErr := <-errCh:
			if runErr != nil {
				t.Fatalf("manager returned error: %v", runErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for manager shutdown")
		}
	})

	poolName := "pool-x"
	objectiveName := "objective-x"
	bindingName := "binding-x"

	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{"app": "model-server"},
				},
			},
		},
	}
	pool.SetGroupVersionKind(inferencePoolGVKs[1])
	pool.SetNamespace(testNamespace)
	pool.SetName(poolName)
	if err := mgr.GetClient().Create(ctx, pool); err != nil {
		t.Fatalf("create pool: %v", err)
	}

	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"poolRef": map[string]any{
					"name":  poolName,
					"group": "inference.networking.x-k8s.io",
				},
			},
		},
	}
	objective.SetGroupVersionKind(inferenceObjectiveGVKs[0])
	objective.SetNamespace(testNamespace)
	objective.SetName(objectiveName)
	if err := mgr.GetClient().Create(ctx, objective); err != nil {
		t.Fatalf("create objective: %v", err)
	}

	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      bindingName,
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			TargetRef:      kleymv1alpha1.InferenceObjectiveTargetRef{Name: objectiveName},
			SelectorSource: kleymv1alpha1.SelectorSourceDerivedFromPool,
			WorkloadSelectorTemplates: []string{
				"k8s:ns:default",
				"k8s:sa:inference-sa",
			},
			Mode: kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
		},
	}
	if err := mgr.GetClient().Create(ctx, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	waitForBindingReady(t, ctx, mgr.GetClient(), types.NamespacedName{Namespace: testNamespace, Name: bindingName})
}

func TestSetupWithManagerFailsWithoutAnySupportedGAIEGVKs(t *testing.T) {
	t.Helper()

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := kleymv1alpha1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add kleym scheme: %v", err)
	}
	registerEnvtestUnstructuredGVK(testScheme, clusterSPIFFEIDGVK)
	for _, gvk := range inferenceObjectiveGVKs {
		registerEnvtestUnstructuredGVK(testScheme, gvk)
	}
	for _, gvk := range inferencePoolGVKs {
		registerEnvtestUnstructuredGVK(testScheme, gvk)
	}

	testEnvironment := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}
	if binDir := getFirstFoundEnvTestBinaryDir(); binDir != "" {
		testEnvironment.BinaryAssetsDirectory = binDir
	}

	cfg, err := testEnvironment.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	t.Cleanup(func() {
		if stopErr := testEnvironment.Stop(); stopErr != nil {
			t.Fatalf("stop envtest: %v", stopErr)
		}
	})

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 testScheme,
		Metrics:                server.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		Controller: config.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	reconciler := &InferenceIdentityBindingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	setupErr := reconciler.SetupWithManager(mgr)
	if setupErr == nil {
		t.Fatalf("SetupWithManager returned nil, want startup error without GAIE CRDs")
	}
	if !strings.Contains(setupErr.Error(), "no supported GAIE GVKs are available") {
		t.Fatalf("unexpected setup error: %v", setupErr)
	}
}

func copyCRDFile(t *testing.T, sourcePath string, destinationDir string) {
	t.Helper()

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read CRD %s: %v", sourcePath, err)
	}

	destinationPath := filepath.Join(destinationDir, filepath.Base(sourcePath))
	if err := os.WriteFile(destinationPath, content, 0o600); err != nil {
		t.Fatalf("write CRD %s: %v", destinationPath, err)
	}
}

func waitForBindingReady(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
	key types.NamespacedName,
) {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for {
		current := &kleymv1alpha1.InferenceIdentityBinding{}
		if err := k8sClient.Get(ctx, key, current); err == nil {
			ready := findCondition(current.Status.Conditions, conditionTypeReady)
			if ready != nil && ready.Status == metav1.ConditionTrue {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for Ready=True, last conditions=%v", current.Status.Conditions)
			}
		} else if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for binding %s: %v", key, err)
		}

		select {
		case <-ctx.Done():
			t.Fatalf("context canceled before binding became ready: %v", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type != conditionType {
			continue
		}
		return &conditions[i]
	}
	return nil
}
