package update

import (
	"errors"
	"os"
	"strconv"
)

// WaitForFlag is what a freshly installed launcher is started with: "--wait-for <process id>" makes it wait until the
// launcher it replaces has gone. A second launcher that started while the first still ran would only hand over to it and
// exit (there is one launcher at a time), so the new one has to wait.
const WaitForFlag = "--wait-for"

// WaitForArg reads "--wait-for <pid>" from the program's arguments (without the program's own name).
func WaitForArg(args []string) (pid int, ok bool) {
	if len(args) != 2 || args[0] != WaitForFlag {
		return 0, false
	}
	pid, err := strconv.Atoi(args[1])
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// RemoveOld deletes the program an update moved aside (exe + ".old"). Having nothing to delete is not an error.
func RemoveOld(exe string) error {
	if err := os.Remove(exe + ".old"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
