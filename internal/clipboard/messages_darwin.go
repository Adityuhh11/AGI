package clipboard

import "errors"

var (
	ErrBlocked  = errors.New("Copied. A password field has the keyboard to itself, which macOS does not let this program type into: press ⌘V yourself.")
	ErrNoTarget = errors.New("Copied. There is no app to paste into: press ⌘V where you want it.")
)
