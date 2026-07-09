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

package main

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func TestManagerCacheOptionsFailForUndeclaredTypedRead(t *testing.T) {
	t.Parallel()

	testScheme := runtime.NewScheme()
	if err := corev1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}

	options := managerCacheOptions()
	options.Scheme = testScheme
	reader, err := cache.New(&rest.Config{Host: "https://localhost"}, options)
	if err != nil {
		t.Fatalf("create cache: %v", err)
	}

	testCases := []struct {
		name string
		read func() error
	}{
		{
			name: "Get",
			read: func() error {
				return reader.Get(context.Background(), types.NamespacedName{}, &corev1.ConfigMap{})
			},
		},
		{
			name: "List",
			read: func() error {
				return reader.List(context.Background(), &corev1.ConfigMapList{})
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.read()
			var notCached *cache.ErrResourceNotCached
			if !errors.As(err, &notCached) {
				t.Fatalf("cache %s error = %v, want ErrResourceNotCached", testCase.name, err)
			}
		})
	}
}
