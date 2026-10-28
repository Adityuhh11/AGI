// Package power says whether the computer runs on its battery.
package power

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var getSystemPowerStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemPowerStatus")

// systemPowerStatus is SYSTEM_POWER_STATUS.
type systemPowerStatus struct {
	ACLineStatus        byte // 0 battery, 1 mains, 255 unknown
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

// OnBattery reports whether the computer is unplugged. When Windows cannot tell (a desktop, a failed call) it is not.
func OnBattery() bool {
	var s systemPowerStatus
	ok, _, _ := getSystemPowerStatus.Call(uintptr(unsafe.Pointer(&s)))
	return ok != 0 && s.ACLineStatus == 0
}
