package clipboard

import "errors"

var (
	ErrBlocked  = errors.New("Copied. That window runs as administrator, which Windows does not let this program type into: press Ctrl+V yourself.")
	ErrNoTarget = errors.New("Copied. There is no window to paste into: press Ctrl+V where you want it.")
)
