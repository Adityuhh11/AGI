#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>
#include "icon_darwin.h"

unsigned char *omniIconPNG(const char *path, int size, int *length) {
	@autoreleasepool {
		NSString *file = [NSString stringWithUTF8String:path];
		NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:file];
		if (icon == nil) {
			return NULL;
		}
		NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithBitmapDataPlanes:NULL
			pixelsWide:size pixelsHigh:size bitsPerSample:8 samplesPerPixel:4 hasAlpha:YES isPlanar:NO
			colorSpaceName:NSCalibratedRGBColorSpace bytesPerRow:0 bitsPerPixel:0];
		if (rep == nil) {
			return NULL;
		}
		NSGraphicsContext *context = [NSGraphicsContext graphicsContextWithBitmapImageRep:rep];
		[NSGraphicsContext saveGraphicsState];
		[NSGraphicsContext setCurrentContext:context];
		[icon drawInRect:NSMakeRect(0, 0, size, size) fromRect:NSZeroRect operation:NSCompositingOperationCopy fraction:1.0];
		[context flushGraphics];
		[NSGraphicsContext restoreGraphicsState];
		NSData *png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
		if (png == nil || png.length == 0) {
			return NULL;
		}
		unsigned char *copy = malloc(png.length);
		memcpy(copy, png.bytes, png.length);
		*length = (int)png.length;
		return copy;
	}
}
