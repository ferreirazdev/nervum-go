package auth

import "testing"

func TestConstantTimeStringEqual(t *testing.T) {
	if !constantTimeStringEqual("same", "same") {
		t.Error("equal strings should match")
	}
	if constantTimeStringEqual("a", "b") {
		t.Error("different strings should not match")
	}
}
