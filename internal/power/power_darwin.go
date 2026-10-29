package power

import (
	"os/exec"
	"strings"
)

// OnBattery reports whether the Mac is unplugged. `pmset -g batt` starts with "Now drawing from 'Battery Power'" or "'AC Power'". When
// it cannot tell (a desktop Mac says AC Power; a failed call says nothing) it is not on battery. It is called about once a minute.
func OnBattery() bool {
	out, err := exec.Command("pmset", "-g", "batt").Output()
	return err == nil && onBattery(string(out))
}

func onBattery(output string) bool {
	return strings.Contains(output, "'Battery Power'")
}
