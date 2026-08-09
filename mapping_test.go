package main

import (
	"math"
	"testing"
)

// --- clampNorm: normal mode ---

func TestClampNorm_NormalMid(t *testing.T) {
	v := clampNorm(512, 0, 1023, "normal", 0)
	if math.Abs(v-0.5) > 0.001 {
		t.Errorf("clampNorm(512, 0, 1023, normal) = %v, want 0.5", v)
	}
}

// --- axisToStick: direction ---

func TestAxisToStick_DirectionBoth(t *testing.T) {
	x, _ := axisToStick("left_stick_x", 512, 0, 1023, "normal", 0, "both")
	if math.Abs(float64(x)) > 200 {
		t.Errorf("both direction at mid should be ~0, got %d", x)
	}
	x, _ = axisToStick("left_stick_x", 1023, 0, 1023, "normal", 0, "both")
	if x < 32000 {
		t.Errorf("both direction at max should be ~32767, got %d", x)
	}
}

func TestAxisToStick_DirectionPositive(t *testing.T) {
	x, _ := axisToStick("left_stick_x", 1023, 0, 1023, "normal", 0, "positive")
	if x < 32000 {
		t.Errorf("positive direction at max should be ~32767, got %d", x)
	}
	x, _ = axisToStick("left_stick_x", 0, 0, 1023, "normal", 0, "positive")
	if x != 0 {
		t.Errorf("positive direction at min should be 0, got %d", x)
	}
}

func TestAxisToStick_DirectionNegative(t *testing.T) {
	x, _ := axisToStick("left_stick_x", 0, 0, 1023, "normal", 0, "negative")
	if x > -32000 {
		t.Errorf("negative direction at min should be ~-32767, got %d", x)
	}
	x, _ = axisToStick("left_stick_x", 1023, 0, 1023, "normal", 0, "negative")
	if x != 0 {
		t.Errorf("negative direction at max should be 0, got %d", x)
	}
}

func TestAxisToStick_DirectionCentered(t *testing.T) {
	x, _ := axisToStick("left_stick_x", 0, 0, 1023, "centered", 0, "positive")
	if x > -32000 {
		t.Errorf("centered with positive dir at min should be ~-32767, got %d", x)
	}
}

// --- axisToTrigger: direction negative ---

func TestAxisToTrigger_DirectionNegative(t *testing.T) {
	v := axisToTrigger(0, 0, 1023, "normal", 0, "negative")
	if v != 255 {
		t.Errorf("negative direction at min should be 255, got %d", v)
	}
	v = axisToTrigger(1023, 0, 1023, "normal", 0, "negative")
	if v != 0 {
		t.Errorf("negative direction at max should be 0, got %d", v)
	}
}

func TestAxisToTrigger_DirectionPositive(t *testing.T) {
	v := axisToTrigger(1023, 0, 1023, "normal", 0, "positive")
	if v != 255 {
		t.Errorf("positive direction at max should be 255, got %d", v)
	}
	v = axisToTrigger(0, 0, 1023, "normal", 0, "positive")
	if v != 0 {
		t.Errorf("positive direction at min should be 0, got %d", v)
	}
}

func TestClampNorm_NormalMin(t *testing.T) {
	v := clampNorm(0, 0, 1023, "normal", 0)
	if v != 0 {
		t.Errorf("clampNorm(0, 0, 1023, normal) = %v, want 0", v)
	}
}

func TestClampNorm_NormalMax(t *testing.T) {
	v := clampNorm(1023, 0, 1023, "normal", 0)
	if v != 1 {
		t.Errorf("clampNorm(1023, 0, 1023, normal) = %v, want 1", v)
	}
}

func TestClampNorm_NormalClampBelow(t *testing.T) {
	v := clampNorm(0, 100, 200, "normal", 0)
	if v != 0 {
		t.Errorf("clampNorm(0, 100, 200, normal) = %v, want 0", v)
	}
}

func TestClampNorm_NormalClampAbove(t *testing.T) {
	v := clampNorm(300, 100, 200, "normal", 0)
	if v != 1 {
		t.Errorf("clampNorm(300, 100, 200, normal) = %v, want 1", v)
	}
}

func TestClampNorm_NormalDeadzone(t *testing.T) {
	z := clampNorm(51, 0, 1023, "normal", 0.1)
	if z != 0 {
		t.Errorf("value in deadzone should be 0, got %v", z)
	}
	full := clampNorm(1023, 0, 1023, "normal", 0.1)
	if math.Abs(full-1.0) > 0.001 {
		t.Errorf("max with deadzone should be 1.0, got %v", full)
	}
}

func TestClampNorm_NormalNonZeroMin(t *testing.T) {
	v := clampNorm(300, 200, 400, "normal", 0)
	if math.Abs(v-0.5) > 0.001 {
		t.Errorf("clampNorm(300, 200, 400, normal) = %v, want 0.5", v)
	}
}

// --- clampNorm: inverted mode ---

func TestClampNorm_InvertedMin(t *testing.T) {
	v := clampNorm(0, 0, 1023, "inverted", 0)
	if v != 1 {
		t.Errorf("clampNorm(0, 0, 1023, inverted) = %v, want 1", v)
	}
}

func TestClampNorm_InvertedMax(t *testing.T) {
	v := clampNorm(1023, 0, 1023, "inverted", 0)
	if v != 0 {
		t.Errorf("clampNorm(1023, 0, 1023, inverted) = %v, want 0", v)
	}
}

func TestClampNorm_InvertedWithDeadzone(t *testing.T) {
	v := clampNorm(0, 0, 1023, "inverted", 0.1)
	if math.Abs(v-1.0) > 0.001 {
		t.Errorf("inverted min with deadzone should be 1.0, got %v", v)
	}
}

// --- clampNorm: centered mode ---

func TestClampNorm_CenteredMidpoint(t *testing.T) {
	v := clampNorm(512, 0, 1023, "centered", 0)
	if math.Abs(v) > 0.001 {
		t.Errorf("centered midpoint should be 0, got %v", v)
	}
}

func TestClampNorm_CenteredMin(t *testing.T) {
	v := clampNorm(0, 0, 1023, "centered", 0)
	if math.Abs(v+1.0) > 0.001 {
		t.Errorf("centered min should be -1, got %v", v)
	}
}

func TestClampNorm_CenteredMax(t *testing.T) {
	v := clampNorm(1023, 0, 1023, "centered", 0)
	if math.Abs(v-1.0) > 0.001 {
		t.Errorf("centered max should be 1, got %v", v)
	}
}

func TestClampNorm_CenteredDeadzone(t *testing.T) {
	// Within 5% of center should be 0
	v := clampNorm(537, 0, 1023, "centered", 0.1) // ~5% right
	if v != 0 {
		t.Errorf("centered within deadzone should be 0, got %v", v)
	}
	// Outside deadzone
	v = clampNorm(700, 0, 1023, "centered", 0.1)
	if v <= 0.2 {
		t.Errorf("centered outside deadzone should be > 0.2, got %v", v)
	}
}

// --- clampNorm: centered_inverted mode ---

func TestClampNorm_CenteredInvertedMin(t *testing.T) {
	v := clampNorm(0, 0, 1023, "centered_inverted", 0)
	if math.Abs(v-1.0) > 0.001 {
		t.Errorf("centered_inverted min should be 1, got %v", v)
	}
}

func TestClampNorm_CenteredInvertedMax(t *testing.T) {
	v := clampNorm(1023, 0, 1023, "centered_inverted", 0)
	if math.Abs(v+1.0) > 0.001 {
		t.Errorf("centered_inverted max should be -1, got %v", v)
	}
}

// --- axisMode resolution ---

func TestAxisMode_Normal(t *testing.T) {
	if mode := axisMode("", false); mode != "normal" {
		t.Errorf("empty mode → %q, want normal", mode)
	}
}

func TestAxisMode_DeprecatedInvert(t *testing.T) {
	if mode := axisMode("", true); mode != "inverted" {
		t.Errorf("empty mode + invert → %q, want inverted", mode)
	}
}

func TestAxisMode_ExplicitMode(t *testing.T) {
	if mode := axisMode("centered", true); mode != "centered" {
		t.Errorf("explicit centered → %q", mode)
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
