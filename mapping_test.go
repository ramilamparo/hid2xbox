package main

import (
	"math"
	"testing"
)

// --- normalizeAxis ---

func TestNormalizeAxis_Basic(t *testing.T) {
	// 0–1023 range: mid = 512 → 0.5
	norm := normalizeAxis(512, 0, 1023, false, 0)
	if math.Abs(norm-0.5) > 0.001 {
		t.Errorf("normalizeAxis(512, 0, 1023) = %v, want ~0.5", norm)
	}
}

func TestNormalizeAxis_Min(t *testing.T) {
	norm := normalizeAxis(0, 0, 1023, false, 0)
	if norm != 0 {
		t.Errorf("normalizeAxis(0, 0, 1023) = %v, want 0", norm)
	}
}

func TestNormalizeAxis_Max(t *testing.T) {
	norm := normalizeAxis(1023, 0, 1023, false, 0)
	if norm != 1 {
		t.Errorf("normalizeAxis(1023, 0, 1023) = %v, want 1", norm)
	}
}

func TestNormalizeAxis_ClampBelow(t *testing.T) {
	norm := normalizeAxis(0, 100, 200, false, 0)
	if norm != 0 {
		t.Errorf("normalizeAxis(0, 100, 200) = %v, want 0 (clamped to min)", norm)
	}
}

func TestNormalizeAxis_ClampAbove(t *testing.T) {
	norm := normalizeAxis(300, 100, 200, false, 0)
	if norm != 1 {
		t.Errorf("normalizeAxis(300, 100, 200) = %v, want 1 (clamped to max)", norm)
	}
}

func TestNormalizeAxis_InvalidRange(t *testing.T) {
	norm := normalizeAxis(50, 100, 100, false, 0)
	if norm != -1 {
		t.Errorf("normalizeAxis(50, 100, 100) = %v, want -1 (invalid range)", norm)
	}
}

func TestNormalizeAxis_Deadzone(t *testing.T) {
	// 10% deadzone: 0.05 → 0, 0.1 → remapped
	z := normalizeAxis(51, 0, 1023, false, 0.1) // raw ~5%
	if z != 0 {
		t.Errorf("value in deadzone should be 0, got %v", z)
	}
	full := normalizeAxis(1023, 0, 1023, false, 0.1)
	if math.Abs(full-1.0) > 0.001 {
		t.Errorf("max with deadzone should be 1.0, got %v", full)
	}
}

func TestNormalizeAxis_Invert(t *testing.T) {
	norm := normalizeAxis(0, 0, 1023, true, 0)
	if norm != 1 {
		t.Errorf("inverted min should be 1, got %v", norm)
	}
	norm = normalizeAxis(1023, 0, 1023, true, 0)
	if norm != 0 {
		t.Errorf("inverted max should be 0, got %v", norm)
	}
}

func TestNormalizeAxis_InvertWithDeadzone(t *testing.T) {
	// Inverted + deadzone: min→1, deadzone near max end
	norm := normalizeAxis(0, 0, 1023, true, 0.1)
	if math.Abs(norm-1.0) > 0.001 {
		t.Errorf("inverted min with deadzone should be 1.0, got %v", norm)
	}
}

func TestNormalizeAxis_NonZeroMin(t *testing.T) {
	norm := normalizeAxis(300, 200, 400, false, 0)
	if math.Abs(norm-0.5) > 0.001 {
		t.Errorf("normalizeAxis(300, 200, 400) = %v, want 0.5", norm)
	}
}

// --- normToStick ---

func TestNormToStick_Center(t *testing.T) {
	if n := normToStick(0.5); n != 0 {
		t.Errorf("normToStick(0.5) = %d, want 0", n)
	}
}

func TestNormToStick_Min(t *testing.T) {
	if n := normToStick(0); n != -32767 {
		t.Errorf("normToStick(0) = %d, want -32767", n)
	}
}

func TestNormToStick_Max(t *testing.T) {
	if n := normToStick(1); n != 32767 {
		t.Errorf("normToStick(1) = %d, want 32767", n)
	}
}

// --- buttonMap coverage ---

func TestButtonMap_AllTargetsExist(t *testing.T) {
	expected := []string{
		"a", "b", "x", "y",
		"left_shoulder", "right_shoulder",
		"start", "back",
		"left_thumb", "right_thumb",
		"dpad_up", "dpad_down", "dpad_left", "dpad_right",
	}
	for _, target := range expected {
		if _, ok := buttonMap[target]; !ok {
			t.Errorf("buttonMap missing target %q", target)
		}
	}
}

func TestApplyButton_UnknownTarget(t *testing.T) {
	// applyButton with unknown target should not panic.
	// (We can't test without a real *x360.Gamepad, but the lookup is safe.)
	_, ok := buttonMap["nonexistent"]
	if ok {
		t.Error("buttonMap should not contain 'nonexistent'")
	}
}

// --- contains ---

func TestContains_Match(t *testing.T) {
	if !contains("Arduino Leonardo", "Leonardo") {
		t.Error("contains(Arduino Leonardo, Leonardo) should be true")
	}
}

func TestContains_NoMatch(t *testing.T) {
	if contains("Arduino Leonardo", "Xbox") {
		t.Error("contains(Arduino Leonardo, Xbox) should be false")
	}
}

func TestContains_EmptySubstr(t *testing.T) {
	if !contains("anything", "") {
		t.Error("contains with empty substring should be true")
	}
}

func TestContains_ExactMatch(t *testing.T) {
	if !contains("Arduino Leonardo", "Arduino Leonardo") {
		t.Error("contains(Arduino Leonardo, Arduino Leonardo) should be true")
	}
}

func TestContains_ShorterString(t *testing.T) {
	if contains("abc", "abcdef") {
		t.Error("contains(abc, abcdef) should be false (substr longer than s)")
	}
}
