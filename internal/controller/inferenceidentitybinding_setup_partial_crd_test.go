package controller

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func TestSetupWithManagerStartsAndReconcilesWithCurrentPoolCRD(t *testing.T) {
	t.Helper()

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := kleymv1alpha1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add kleym scheme: %v", err)
	}
	registerEnvtestUnstructuredGVK(testScheme, clusterSPIFFEIDGVK)
	for _, gvk := range inferencePoolGVKs {
		registerEnvtestUnstructuredGVK(testScheme, gvk)
	}

	crdDir := t.TempDir()
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
	apiClient, err := client.New(cfg, client.Options{Scheme: testScheme})
	if err != nil {
		t.Fatalf("create direct API client: %v", err)
	}
	assertLegacyPoolGroupRejectedByCRD(t, context.Background(), apiClient)
	assertServiceAccountNameValidationAtCRDAdmission(t, context.Background(), apiClient)
	assertIdentityBoundaryValidationAtCRDAdmission(t, context.Background(), apiClient)

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: mgr.GetClient(),
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		t.Fatalf("setup controller with partial CRDs: %v", err)
	}

	if len(reconciler.availablePoolGVKs) != 1 {
		t.Fatalf("availablePoolGVKs = %v, want current GAIE pool GVK", reconciler.availablePoolGVKs)
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

	poolName := "pool-current"
	bindingName := "binding-current"

	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{"app": "model-server"},
				},
			},
		},
	}
	pool.SetGroupVersionKind(inferencePoolGVKs[0])
	pool.SetNamespace(testNamespace)
	pool.SetName(poolName)
	if err := apiClient.Create(ctx, pool); err != nil {
		t.Fatalf("create pool: %v", err)
	}

	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      bindingName,
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: poolName, Group: "inference.networking.k8s.io"},
			ServiceAccountName: "inference-sa",
			IdentityBoundary:   testIdentityBoundary,
		},
	}
	if err := apiClient.Create(ctx, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	ready := waitForBindingReady(t, ctx, mgr.GetClient(), types.NamespacedName{Namespace: testNamespace, Name: bindingName})
	recordedName := ready.Status.OwnedClusterSPIFFEIDName
	if recordedName == "" {
		t.Fatal("ownedClusterSPIFFEIDName was not recorded")
	}

	created := fetchClusterSPIFFEID(t, ctx, apiClient, recordedName)
	if err := apiClient.Delete(ctx, created); err != nil {
		t.Fatalf("delete recorded ClusterSPIFFEID: %v", err)
	}

	recreated := waitForClusterSPIFFEIDRecreated(t, ctx, apiClient, recordedName, created.GetUID())
	waitForBindingReady(t, ctx, apiClient, types.NamespacedName{Namespace: testNamespace, Name: bindingName})

	desiredSpec, _, err := unstructured.NestedMap(recreated.Object, "spec")
	if err != nil {
		t.Fatalf("read recreated ClusterSPIFFEID spec: %v", err)
	}
	drifted := recreated.DeepCopy()
	drifted.Object["spec"] = map[string]any{
		"spiffeIDTemplate":          "spiffe://drifted.example/workload",
		"podSelector":               map[string]any{"matchLabels": map[string]any{"app": "drifted"}},
		"workloadSelectorTemplates": []any{"k8s:ns:default", "k8s:sa:drifted"},
	}
	if err := apiClient.Update(ctx, drifted); err != nil {
		t.Fatalf("mutate recorded ClusterSPIFFEID spec: %v", err)
	}

	waitForClusterSPIFFEIDSpec(t, ctx, apiClient, recordedName, desiredSpec)
	waitForBindingReady(t, ctx, apiClient, types.NamespacedName{Namespace: testNamespace, Name: bindingName})
}

func TestSetupWithManagerSkipsClusterSPIFFEIDWatchWhenCRDMissing(t *testing.T) {
	t.Helper()

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := kleymv1alpha1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add kleym scheme: %v", err)
	}
	registerEnvtestUnstructuredGVK(testScheme, clusterSPIFFEIDGVK)
	for _, gvk := range inferencePoolGVKs {
		registerEnvtestUnstructuredGVK(testScheme, gvk)
	}

	crdDir := t.TempDir()
	copyCRDFile(t, filepath.Join("testdata", "crds", "inference.networking.k8s.io_inferencepools.yaml"), crdDir)

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

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: mgr.GetClient(),
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		t.Fatalf("setup controller without ClusterSPIFFEID CRD: %v", err)
	}

	apiClient, err := client.New(cfg, client.Options{Scheme: testScheme})
	if err != nil {
		t.Fatalf("create direct API client: %v", err)
	}

	pool := newTestPool()
	pool.SetName("pool-without-managed-crd")
	if err := apiClient.Create(context.Background(), pool); err != nil {
		t.Fatalf("create pool: %v", err)
	}

	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "binding-without-managed-crd",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: pool.GetName(), Group: "inference.networking.k8s.io"},
			ServiceAccountName: "inference-sa",
			IdentityBoundary:   testIdentityBoundary,
		},
	}
	if err := apiClient.Create(context.Background(), binding); err != nil {
		t.Fatalf("create binding: %v", err)
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
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		t.Fatalf("timed out waiting for manager cache sync")
	}

	result, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("reconcile without ClusterSPIFFEID CRD returned error: %v", err)
	}
	if result.RequeueAfter != infraNotReadyRequeueAfter {
		t.Fatalf("requeueAfter = %s, want %s", result.RequeueAfter, infraNotReadyRequeueAfter)
	}

	current := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := apiClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: binding.Name}, current); err != nil {
		t.Fatalf("fetch binding after transient managed-output failure: %v", err)
	}
	assertPrimaryFailureCondition(t, current, conditionTypeRenderFailure, conditionReasonClusterSPIFFEIDCRDMissing)
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

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: mgr.GetClient(),
	}
	setupErr := reconciler.SetupWithManager(mgr)
	if setupErr == nil {
		t.Fatalf("SetupWithManager returned nil, want startup error without GAIE CRDs")
	}
	if !strings.Contains(setupErr.Error(), "no supported GAIE InferencePool GVKs are available") {
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

func assertLegacyPoolGroupRejectedByCRD(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
) {
	t.Helper()

	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "legacy-pool-group-rejected",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef: kleymv1alpha1.InferencePoolTargetRef{
				Name:  "pool-current",
				Group: "inference.networking.x-k8s.io",
			},
			ServiceAccountName: "inference-sa",
			IdentityBoundary:   testIdentityBoundary,
		},
	}
	err := k8sClient.Create(ctx, binding)
	if !apierrors.IsInvalid(err) {
		t.Fatalf("create binding with legacy pool group error = %v, want Invalid", err)
	}
}

// assertServiceAccountNameValidationAtCRDAdmission verifies CRD admission matches the DNS-1123 service account contract.
func assertServiceAccountNameValidationAtCRDAdmission(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
) {
	t.Helper()

	invalidBinding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "invalid-service-account-rejected",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-current"},
			ServiceAccountName: "Invalid_ServiceAccount",
			IdentityBoundary:   testIdentityBoundary,
		},
	}
	if err := k8sClient.Create(ctx, invalidBinding); !apierrors.IsInvalid(err) {
		t.Fatalf("create binding with invalid service account error = %v, want Invalid", err)
	}

	validBinding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "dns-subdomain-service-account-admitted",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-current"},
			ServiceAccountName: "inference.service-account",
			IdentityBoundary:   testIdentityBoundary,
		},
	}
	if err := k8sClient.Create(ctx, validBinding); err != nil {
		t.Fatalf("create binding with DNS-1123 subdomain service account: %v", err)
	}
	if err := k8sClient.Delete(ctx, validBinding); err != nil {
		t.Fatalf("delete admitted binding: %v", err)
	}
}

// assertIdentityBoundaryValidationAtCRDAdmission verifies required boundary fields and formats.
func assertIdentityBoundaryValidationAtCRDAdmission(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
) {
	t.Helper()

	cases := map[string]kleymv1alpha1.IdentityBoundary{
		"missing":        {},
		"unreserved-key": {LabelKey: "example.com/variant", LabelValue: "prefill"},
		"malformed-key":  {LabelKey: "identity.kleym.sonda.red/bad key", LabelValue: "prefill"},
		"empty-value":    {LabelKey: "identity.kleym.sonda.red/variant"},
		"malformed-value": {
			LabelKey:   "identity.kleym.sonda.red/variant",
			LabelValue: "bad/value",
		},
	}
	for name, boundary := range cases {
		binding := &kleymv1alpha1.InferenceIdentityBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "boundary-" + name},
			Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
				PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-current"},
				ServiceAccountName: "inference-sa",
				IdentityBoundary:   boundary,
			},
		}
		if err := k8sClient.Create(ctx, binding); !apierrors.IsInvalid(err) {
			t.Fatalf("create binding with %s boundary error = %v, want Invalid", name, err)
		}
	}
}

func waitForBindingReady(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
	key types.NamespacedName,
) *kleymv1alpha1.InferenceIdentityBinding {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for {
		current := &kleymv1alpha1.InferenceIdentityBinding{}
		if err := k8sClient.Get(ctx, key, current); err == nil {
			ready := findCondition(current.Status.Conditions, conditionTypeReady)
			if ready != nil && ready.Status == metav1.ConditionTrue {
				return current
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

func fetchClusterSPIFFEID(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
	name string,
) *unstructured.Unstructured {
	t.Helper()

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, current); err != nil {
		t.Fatalf("fetch ClusterSPIFFEID %q: %v", name, err)
	}
	return current
}

func waitForClusterSPIFFEIDRecreated(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
	name string,
	deletedUID types.UID,
) *unstructured.Unstructured {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for {
		current := &unstructured.Unstructured{}
		current.SetGroupVersionKind(clusterSPIFFEIDGVK)
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, current)
		if err == nil && current.GetUID() != deletedUID {
			return current
		}
		if err != nil && !apierrors.IsNotFound(err) {
			t.Fatalf("wait for ClusterSPIFFEID %q recreation: %v", name, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for ClusterSPIFFEID %q recreation", name)
		}

		select {
		case <-ctx.Done():
			t.Fatalf("context canceled before ClusterSPIFFEID recreation: %v", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func waitForClusterSPIFFEIDSpec(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
	name string,
	want map[string]any,
) {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for {
		current := &unstructured.Unstructured{}
		current.SetGroupVersionKind(clusterSPIFFEIDGVK)
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, current)
		if err == nil {
			got, _, nestedErr := unstructured.NestedMap(current.Object, "spec")
			if nestedErr != nil {
				t.Fatalf("read ClusterSPIFFEID %q spec: %v", name, nestedErr)
			}
			if reflect.DeepEqual(got, want) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for ClusterSPIFFEID %q spec correction: got %#v want %#v", name, got, want)
			}
		} else if !apierrors.IsNotFound(err) {
			t.Fatalf("wait for ClusterSPIFFEID %q spec correction: %v", name, err)
		} else if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for ClusterSPIFFEID %q spec correction: %v", name, err)
		}

		select {
		case <-ctx.Done():
			t.Fatalf("context canceled before ClusterSPIFFEID spec correction: %v", ctx.Err())
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
