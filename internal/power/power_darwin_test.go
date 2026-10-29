package power

import "testing"

func TestTheFirstLineOfPmsetSaysWhereThePowerComesFrom(t *testing.T) {
	tests := map[string]bool{
		"Now drawing from 'Battery Power'\n -InternalBattery-0 (id=1) 87%; discharging; 5:12 remaining present: true": true,
		"Now drawing from 'AC Power'\n -InternalBattery-0 (id=1) 100%; charged; 0:00 remaining present: true":       false,
		"Now drawing from 'AC Power'\n":                                      false,
		"":                                                                    false,
	}
	for output, want := range tests {
		if got := onBattery(output); got != want {
			t.Errorf("onBattery(%q) = %v, want %v", output, got, want)
		}
	}
}
