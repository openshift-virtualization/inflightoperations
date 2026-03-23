package settings

import (
	"testing"
	"time"
)

func TestLookupInt(t *testing.T) {
	t.Run("default when unset", func(t *testing.T) {
		val, err := LookupInt("TEST_LOOKUP_INT_UNSET", 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 42 {
			t.Fatalf("expected 42, got %d", val)
		}
	})
	t.Run("value from env", func(t *testing.T) {
		t.Setenv("TEST_LOOKUP_INT_SET", "99")
		val, err := LookupInt("TEST_LOOKUP_INT_SET", 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 99 {
			t.Fatalf("expected 99, got %d", val)
		}
	})
	t.Run("error on non-numeric", func(t *testing.T) {
		t.Setenv("TEST_LOOKUP_INT_BAD", "abc")
		_, err := LookupInt("TEST_LOOKUP_INT_BAD", 42)
		if err == nil {
			t.Fatal("expected error for non-numeric value")
		}
	})
}

func TestLookupBool(t *testing.T) {
	t.Run("default when unset", func(t *testing.T) {
		val, err := LookupBool("TEST_LOOKUP_BOOL_UNSET", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != true {
			t.Fatal("expected true")
		}
	})
	t.Run("true from env", func(t *testing.T) {
		t.Setenv("TEST_LOOKUP_BOOL_SET", "true")
		val, err := LookupBool("TEST_LOOKUP_BOOL_SET", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != true {
			t.Fatal("expected true")
		}
	})
	t.Run("false from env", func(t *testing.T) {
		t.Setenv("TEST_LOOKUP_BOOL_SET2", "false")
		val, err := LookupBool("TEST_LOOKUP_BOOL_SET2", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != false {
			t.Fatal("expected false")
		}
	})
	t.Run("error on invalid", func(t *testing.T) {
		t.Setenv("TEST_LOOKUP_BOOL_BAD", "notabool")
		_, err := LookupBool("TEST_LOOKUP_BOOL_BAD", false)
		if err == nil {
			t.Fatal("expected error for invalid bool value")
		}
	})
}

func TestLookupSeconds(t *testing.T) {
	t.Run("default when unset", func(t *testing.T) {
		val, err := LookupSeconds("TEST_LOOKUP_SEC_UNSET", 30)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 30*time.Second {
			t.Fatalf("expected 30s, got %v", val)
		}
	})
	t.Run("value from env", func(t *testing.T) {
		t.Setenv("TEST_LOOKUP_SEC_SET", "60")
		val, err := LookupSeconds("TEST_LOOKUP_SEC_SET", 30)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 60*time.Second {
			t.Fatalf("expected 60s, got %v", val)
		}
	})
	t.Run("error on non-numeric", func(t *testing.T) {
		t.Setenv("TEST_LOOKUP_SEC_BAD", "xyz")
		_, err := LookupSeconds("TEST_LOOKUP_SEC_BAD", 30)
		if err == nil {
			t.Fatal("expected error for non-numeric value")
		}
	})
}

func TestSettingsLoad(t *testing.T) {
	t.Setenv(EnvDebounceThreshold, "10")
	t.Setenv(EnvInformerSyncTimeout, "20")
	t.Setenv(EnvK8SAPITimeout, "15")
	t.Setenv(EnvK8SInformerResync, "45")
	t.Setenv(EnvRetainCompletedIFOs, "true")
	t.Setenv(EnvRequeueInterval, "120")
	t.Setenv(EnvOperatorVersion, "1.0.0")

	var s ControllerSettings
	err := s.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DebounceThreshold != 10*time.Second {
		t.Errorf("DebounceThreshold: expected 10s, got %v", s.DebounceThreshold)
	}
	if s.InformerSyncTimeout != 20*time.Second {
		t.Errorf("InformerSyncTimeout: expected 20s, got %v", s.InformerSyncTimeout)
	}
	if s.K8SAPITimeout != 15*time.Second {
		t.Errorf("K8SAPITimeout: expected 15s, got %v", s.K8SAPITimeout)
	}
	if s.K8SInformerResync != 45*time.Second {
		t.Errorf("K8SInformerResync: expected 45s, got %v", s.K8SInformerResync)
	}
	if s.RetainCompletedIFOs != true {
		t.Error("RetainCompletedIFOs: expected true")
	}
	if s.RequeueInterval != 120*time.Second {
		t.Errorf("RequeueInterval: expected 120s, got %v", s.RequeueInterval)
	}
	if s.OperatorVersion != "1.0.0" {
		t.Errorf("OperatorVersion: expected 1.0.0, got %s", s.OperatorVersion)
	}
}
