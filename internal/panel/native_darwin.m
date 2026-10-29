#import <Cocoa/Cocoa.h>
#include "native_darwin.h"

// AppKit may only be used from the main thread; the hotkey and the page call in from others.
static void onMain(void (^block)(void)) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_sync(dispatch_get_main_queue(), block);
	}
}

static NSWindow *windowOf(uintptr_t window) {
	return (__bridge NSWindow *)(void *)window;
}

// The height of the primary screen (the one with the menu bar, first in the list): Cocoa's y grows upward from its bottom edge.
static CGFloat primaryHeight(void) {
	NSArray<NSScreen *> *screens = [NSScreen screens];
	return screens.count > 0 ? screens[0].frame.size.height : 0;
}

int omniFrontmostPID(void) {
	__block int pid = 0;
	onMain(^{
		NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
		pid = app ? app.processIdentifier : 0;
	});
	return pid;
}

int omniIsRunning(int pid) {
	return [NSRunningApplication runningApplicationWithProcessIdentifier:pid] != nil;
}

// Brings the application forward, but only when it is not the one in front already: activating it raises every one of its windows, which
// would put two Safari windows above an Xcode window each time the bar is closed with Escape.
void omniActivate(int pid) {
	onMain(^{
		NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
		if (front != nil && front.processIdentifier == pid) {
			return;
		}
		[[NSRunningApplication runningApplicationWithProcessIdentifier:pid] activateWithOptions:NSApplicationActivateAllWindows];
	});
}

// orderFrontRegardless shows the panel without making this program the active one.
void omniOrderFront(uintptr_t window) {
	onMain(^{ [windowOf(window) orderFrontRegardless]; });
}

void omniOrderOut(uintptr_t window) {
	onMain(^{ [windowOf(window) orderOut:nil]; });
}

// The usable area (menu bar and Dock left out) of the screen that holds the mouse.
void omniScreenUnderMouse(double *x, double *y, double *w, double *h) {
	onMain(^{
		NSPoint mouse = [NSEvent mouseLocation];
		NSScreen *found = nil;
		for (NSScreen *screen in [NSScreen screens]) {
			if (NSPointInRect(mouse, screen.frame)) {
				found = screen;
				break;
			}
		}
		if (found == nil) {
			found = [NSScreen mainScreen] ?: [NSScreen screens].firstObject;
		}
		NSRect area = found.visibleFrame;
		*x = area.origin.x;
		*y = primaryHeight() - (area.origin.y + area.size.height);
		*w = area.size.width;
		*h = area.size.height;
	});
}

void omniWindowFrame(uintptr_t window, double *x, double *y, double *w, double *h) {
	onMain(^{
		NSRect frame = windowOf(window).frame;
		*x = frame.origin.x;
		*y = primaryHeight() - (frame.origin.y + frame.size.height);
		*w = frame.size.width;
		*h = frame.size.height;
	});
}

void omniSetFrame(uintptr_t window, double x, double y, double w, double h) {
	onMain(^{
		[windowOf(window) setFrame:NSMakeRect(x, primaryHeight() - y - h, w, h) display:NO];
	});
}

// Changes the height and keeps the top edge where it is.
void omniSetHeight(uintptr_t window, double h) {
	onMain(^{
		NSWindow *win = windowOf(window);
		NSRect frame = win.frame;
		frame.origin.y += frame.size.height - h;
		frame.size.height = h;
		[win setFrame:frame display:YES];
	});
}

