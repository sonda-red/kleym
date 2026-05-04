package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestReconcileLogsStructuredSuccessPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger, logs := newRecordingLogger()
	ctx = logf.IntoContext(ctx, logger)

	scheme := newCollisionTestScheme(t)
	binding := newPoolOnlyBinding("binding-log-success", "objective-a")
	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), newTestObjective("objective-a"), binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(binding),
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	logs.requireEntry(t, "starting InferenceIdentityBinding reconcile", map[string]string{
		logKeyBinding:   "default/binding-log-success",
		logKeyNamespace: testNamespace,
		logKeyName:      "binding-log-success",
	})
	logs.requireEntry(t, "resolved target InferenceObjective", map[string]string{
		logKeyTargetRef: "objective-a",
		logKeyObjective: "default/objective-a",
	})
	logs.requireEntry(t, "resolved target InferencePool", map[string]string{
		logKeyPool: "default/pool-a",
	})
	logs.requireEntry(t, "rendered identity from inference intent", map[string]string{
		logKeyMode:      string(kleymv1alpha1.InferenceIdentityBindingModePoolOnly),
		logKeyObjective: "objective-a",
		logKeyPool:      "pool-a",
		logKeySpiffeID:  "spiffe://kleym.sonda.red/ns/default/pool/pool-a",
	})
	logs.requireEntry(t, "creating managed ClusterSPIFFEID", map[string]string{
		logKeyMode:     string(kleymv1alpha1.InferenceIdentityBindingModePoolOnly),
		logKeySpiffeID: "spiffe://kleym.sonda.red/ns/default/pool/pool-a",
	})
	logs.requireEntry(t, "applied success status", map[string]string{
		logKeyCondition: conditionTypeReady,
		logKeyReason:    "Reconciled",
	})
	logs.requireEntry(t, "finished InferenceIdentityBinding reconcile", map[string]string{
		logKeyRequeueAfter: "0s",
	})
}

func TestReconcileLogsFailureStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger, logs := newRecordingLogger()
	ctx = logf.IntoContext(ctx, logger)

	scheme := newCollisionTestScheme(t)
	binding := newPerObjectiveBinding("binding-log-failure", "missing-objective")
	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(binding),
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	logs.requireEntry(t, "cleaning up managed ClusterSPIFFEIDs after reconcile failure", map[string]string{
		logKeyCondition: conditionTypeInvalidRef,
		logKeyReason:    "TargetObjectiveNotFound",
	})
	logs.requireEntry(t, "applied failure status", map[string]string{
		logKeyCondition: conditionTypeInvalidRef,
		logKeyReason:    "TargetObjectiveNotFound",
	})
	logs.requireEntry(t, "finished InferenceIdentityBinding reconcile", map[string]string{
		logKeyBinding: "default/binding-log-failure",
	})
}

func TestReconcileClusterSPIFFEIDsLogsApplyDecisions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger, logs := newRecordingLogger()
	ctx = logf.IntoContext(ctx, logger)

	scheme := newCollisionTestScheme(t)
	binding := newPoolOnlyBinding("binding-log-apply", "objective-a")
	identity := renderedIdentity{
		Name:     "desired-clusterspiffeid",
		Mode:     kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
		SpiffeID: "spiffe://kleym.sonda.red/ns/default/pool/pool-a",
		Selectors: []string{
			"k8s:ns:default",
			"k8s:pod-label:app:model-server",
			"k8s:sa:inference-sa",
		},
		PodSelector: map[string]any{
			"matchLabels": map[string]any{"app": "model-server"},
		},
		ObjectiveRef: "objective-a",
		PoolRef:      "pool-a",
	}

	drifted := desiredClusterSPIFFEID(binding, identity)
	drifted.Object["spec"] = map[string]any{"spiffeIDTemplate": "spiffe://wrong"}

	staleIdentity := identity
	staleIdentity.Name = "stale-clusterspiffeid"
	stale := desiredClusterSPIFFEID(binding, staleIdentity)

	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(binding, drifted, stale).
			Build(),
		Scheme: scheme,
	}

	if err := reconciler.reconcileClusterSPIFFEIDs(ctx, binding, []renderedIdentity{identity}); err != nil {
		t.Fatalf("reconcileClusterSPIFFEIDs returned error: %v", err)
	}

	logs.requireEntry(t, "updating drifted managed ClusterSPIFFEID", map[string]string{
		logKeyClusterSPIFFEID: "desired-clusterspiffeid",
		logKeySpiffeID:        identity.SpiffeID,
	})
	logs.requireEntry(t, "deleting stale managed ClusterSPIFFEID", map[string]string{
		logKeyClusterSPIFFEID: "stale-clusterspiffeid",
	})
}

type recordedLogEntry struct {
	message string
	values  map[string]string
}

type recordedLogs struct {
	mu      sync.Mutex
	entries []recordedLogEntry
}

func newRecordingLogger() (logr.Logger, *recordedLogs) {
	logs := &recordedLogs{}
	return logr.New(&recordingLogSink{logs: logs}), logs
}

func (l *recordedLogs) add(message string, keyValues []any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, recordedLogEntry{
		message: message,
		values:  keyValueStrings(keyValues),
	})
}

func (l *recordedLogs) requireEntry(t *testing.T, message string, expected map[string]string) {
	t.Helper()

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if entry.message != message {
			continue
		}
		matches := true
		for key, value := range expected {
			if entry.values[key] != value {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}

	t.Fatalf("missing log entry %q with values %v\nlogs:\n%s", message, expected, l.dumpLocked())
}

func (l *recordedLogs) dumpLocked() string {
	var builder strings.Builder
	for _, entry := range l.entries {
		builder.WriteString(entry.message)
		builder.WriteString(" ")
		fmt.Fprint(&builder, entry.values)
		builder.WriteString("\n")
	}
	return builder.String()
}

type recordingLogSink struct {
	logs   *recordedLogs
	values []any
}

func (s *recordingLogSink) Init(logr.RuntimeInfo) {}

func (s *recordingLogSink) Enabled(int) bool {
	return true
}

func (s *recordingLogSink) Info(_ int, message string, keyValues ...any) {
	s.logs.add(message, appendKeyValues(s.values, keyValues))
}

func (s *recordingLogSink) Error(err error, message string, keyValues ...any) {
	allValues := appendKeyValues(s.values, keyValues)
	allValues = append(allValues, "error", err)
	s.logs.add(message, allValues)
}

func (s *recordingLogSink) WithValues(keyValues ...any) logr.LogSink {
	return &recordingLogSink{
		logs:   s.logs,
		values: appendKeyValues(s.values, keyValues),
	}
}

func (s *recordingLogSink) WithName(string) logr.LogSink {
	return &recordingLogSink{
		logs:   s.logs,
		values: appendKeyValues(s.values, nil),
	}
}

func appendKeyValues(base []any, extra []any) []any {
	combined := make([]any, 0, len(base)+len(extra))
	combined = append(combined, base...)
	combined = append(combined, extra...)
	return combined
}

func keyValueStrings(keyValues []any) map[string]string {
	values := make(map[string]string, len(keyValues)/2)
	for i := 0; i+1 < len(keyValues); i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			continue
		}
		values[key] = fmt.Sprint(keyValues[i+1])
	}
	return values
}
