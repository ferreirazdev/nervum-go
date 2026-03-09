package types

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
)

func TestJSONB_Value_Unit(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var j JSONB = nil
		v, err := j.Value()
		if err != nil {
			t.Fatalf("Value(): %v", err)
		}
		if v != nil {
			t.Errorf("Value() = %v, want nil", v)
		}
	})

	t.Run("empty map", func(t *testing.T) {
		j := JSONB{}
		v, err := j.Value()
		if err != nil {
			t.Fatalf("Value(): %v", err)
		}
		b, ok := v.([]byte)
		if !ok {
			t.Fatalf("Value() type = %T, want []byte", v)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if len(out) != 0 {
			t.Errorf("expected empty object, got %v", out)
		}
	})

	t.Run("non-empty map", func(t *testing.T) {
		j := JSONB{"a": "b", "n": float64(1)}
		v, err := j.Value()
		if err != nil {
			t.Fatalf("Value(): %v", err)
		}
		b, ok := v.([]byte)
		if !ok {
			t.Fatalf("Value() type = %T, want []byte", v)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if out["a"] != "b" || out["n"] != float64(1) {
			t.Errorf("got %v", out)
		}
	})
}

func TestJSONB_Scan_Unit(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var j JSONB
		if err := j.Scan(nil); err != nil {
			t.Fatalf("Scan(nil): %v", err)
		}
		if j != nil {
			t.Errorf("Scan(nil): got %v, want nil", j)
		}
	})

	t.Run("valid bytes", func(t *testing.T) {
		var j JSONB
		if err := j.Scan([]byte(`{"x":"y"}`)); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if j["x"] != "y" {
			t.Errorf("got %v", j)
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		var j JSONB
		err := j.Scan("not bytes")
		if err == nil {
			t.Fatal("expected error for invalid type")
		}
		if err.Error() != "invalid type for JSONB" {
			t.Errorf("got error %q", err.Error())
		}
	})
}

func TestJSONB_RoundTrip_Unit(t *testing.T) {
	in := JSONB{"k": "v", "num": float64(42)}
	v, err := in.Value()
	if err != nil {
		t.Fatalf("Value(): %v", err)
	}
	var out JSONB
	if err := out.Scan(v); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if out["k"] != "v" || out["num"] != float64(42) {
		t.Errorf("round-trip got %v", out)
	}
}

func TestJSONB_Value_ImplementsDriverValuer(t *testing.T) {
	var _ driver.Valuer = JSONB(nil)
}
