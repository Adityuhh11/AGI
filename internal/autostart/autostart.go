// Package autostart starts the launcher when its user signs in. Enabled and Set are the same on both systems; how it is
// done is not:
//
//   - Windows (autostart_windows.go): the per-user Run key, which needs no administrator rights and which Task Manager's
//     Startup apps list shows and can switch off.
//   - macOS (autostart_darwin.go): a login item, which System Settings > General > Login Items shows and can switch off.
package autostart
