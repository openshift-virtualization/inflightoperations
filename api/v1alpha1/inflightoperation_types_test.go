package v1alpha1

import (
	"testing"
	"time"

	"github.com/openshift-virtualization/inflightoperations/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeSubject(name, namespace string, generation int64) *Subject {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":       name,
				"namespace":  namespace,
				"generation": generation,
			},
		},
	}
}

func TestComplete(t *testing.T) {
	t.Run("false when Completed is nil", func(t *testing.T) {
		ifo := &InFlightOperation{}
		if ifo.Complete() {
			t.Fatal("expected false when Completed is nil")
		}
	})
	t.Run("true when Completed is set", func(t *testing.T) {
		now := metav1.Now()
		ifo := &InFlightOperation{
			Status: InFlightOperationStatus{
				Completed: &now,
			},
		}
		if !ifo.Complete() {
			t.Fatal("expected true when Completed is set")
		}
	})
}

func TestPastDebounceThreshold(t *testing.T) {
	settings.Settings.DebounceThreshold = 5 * time.Second

	t.Run("false when not complete", func(t *testing.T) {
		ifo := &InFlightOperation{}
		if ifo.PastDebounceThreshold() {
			t.Fatal("expected false when not complete")
		}
	})
	t.Run("false when within threshold", func(t *testing.T) {
		now := metav1.Now()
		ifo := &InFlightOperation{
			Status: InFlightOperationStatus{
				Completed: &now,
			},
		}
		if ifo.PastDebounceThreshold() {
			t.Fatal("expected false when within debounce threshold")
		}
	})
	t.Run("true when past threshold", func(t *testing.T) {
		past := metav1.NewTime(time.Now().Add(-10 * time.Second))
		ifo := &InFlightOperation{
			Status: InFlightOperationStatus{
				Completed: &past,
			},
		}
		if !ifo.PastDebounceThreshold() {
			t.Fatal("expected true when past debounce threshold")
		}
	})
}

func TestMarkCompleted(t *testing.T) {
	subject := makeSubject("my-vm", "default", 3)
	ifo := &InFlightOperation{
		Status: InFlightOperationStatus{
			Phase: OperationPhaseActive,
		},
	}

	ifo.MarkCompleted(subject)

	if ifo.Status.Phase != OperationPhaseCompleted {
		t.Errorf("expected phase Completed, got %s", ifo.Status.Phase)
	}
	if ifo.Status.Completed == nil {
		t.Fatal("expected Completed timestamp to be set")
	}
	if ifo.Status.SubjectGeneration != 3 {
		t.Errorf("expected SubjectGeneration 3, got %d", ifo.Status.SubjectGeneration)
	}
}

func TestMarkDetection(t *testing.T) {
	subject := makeSubject("my-vm", "default", 5)
	completed := metav1.Now()
	ifo := &InFlightOperation{
		Status: InFlightOperationStatus{
			Phase:     OperationPhaseCompleted,
			Completed: &completed,
		},
	}

	ifo.MarkDetection(subject, []string{"ruleset-a", "ruleset-b"})

	if ifo.Status.Phase != OperationPhaseActive {
		t.Errorf("expected phase Active, got %s", ifo.Status.Phase)
	}
	if ifo.Status.Completed != nil {
		t.Fatal("expected Completed to be nil after MarkDetection")
	}
	if ifo.Status.LastDetected == nil {
		t.Fatal("expected LastDetected to be set")
	}
	if len(ifo.Status.DetectedBy) != 2 || ifo.Status.DetectedBy[0] != "ruleset-a" {
		t.Errorf("unexpected DetectedBy: %v", ifo.Status.DetectedBy)
	}
	if ifo.Status.SubjectGeneration != 5 {
		t.Errorf("expected SubjectGeneration 5, got %d", ifo.Status.SubjectGeneration)
	}
}
