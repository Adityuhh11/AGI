package dock

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// AppKit only from the main thread.
static void omniSetRegular(int regular) {
	void (^work)(void) = ^{
		if (regular) {
			[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
			[NSApp activateIgnoringOtherApps:YES]; // the window that was just opened comes to the front
		} else {
			[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
		}
	};
	if ([NSThread isMainThread]) {
		work();
	} else {
		dispatch_sync(dispatch_get_main_queue(), work);
	}
}
*/
import "C"

// setRegular gives the program a Dock icon, a ⌘Tab entry and a menu bar (true), or takes them away (false).
func setRegular(regular bool) {
	if regular {
		C.omniSetRegular(1)
	} else {
		C.omniSetRegular(0)
	}
}
