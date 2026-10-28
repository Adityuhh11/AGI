package power

import "testing"

// The answer depends on the machine; this only proves the call works and does not panic.
func TestOnBatteryAnswers(t *testing.T) {
	t.Logf("on battery: %v", OnBattery())
}
