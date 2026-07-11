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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
)

func testOperatorConfig() OperatorConfig {
	return OperatorConfig{TrustDomain: "kleym.sonda.red"}
}

func TestMain(m *testing.M) {
	logf.SetLogger(zap.New(zap.WriteTo(os.Stderr), zap.UseDevMode(true)))

	if err := kleymv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		fmt.Fprintf(os.Stderr, "add kleym scheme: %v\n", err)
		os.Exit(1)
	}
	registerEnvtestUnstructuredGVK(scheme.Scheme, clusterSPIFFEIDGVK)
	for _, gvk := range inferencePoolGVKs {
		registerEnvtestUnstructuredGVK(scheme.Scheme, gvk)
	}

	// +kubebuilder:scaffold:scheme

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("testdata", "crds"),
		},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	cfg, err := testEnv.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "start envtest: %v\n", err)
		os.Exit(1)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create envtest client: %v\n", err)
		_ = testEnv.Stop()
		os.Exit(1)
	}

	exitCode := m.Run()
	err = testEnv.Stop()
	if os.Getenv("GITHUB_ACTIONS") == "true" &&
		err != nil &&
		strings.Contains(err.Error(), "unable to signal for process") &&
		strings.Contains(err.Error(), "permission denied") {
		fmt.Fprintln(os.Stderr, "Ignoring envtest stop permission error in GitHub Actions")
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "stop envtest: %v\n", err)
		exitCode = 1
	}
	os.Exit(exitCode)
}

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

func registerEnvtestUnstructuredGVK(s *runtime.Scheme, gvk schema.GroupVersionKind) {
	s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
}
