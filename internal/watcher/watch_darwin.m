#import <Foundation/Foundation.h>
#import <CoreServices/CoreServices.h>
#include "watch_darwin.h"

static void onEvents(ConstFSEventStreamRef stream, void *info, size_t count, void *eventPaths,
		const FSEventStreamEventFlags flags[], const FSEventStreamEventId ids[]) {
	char **paths = (char **)eventPaths;
	for (size_t i = 0; i < count; i++) {
		goWatchEvent((long)info, paths[i], (unsigned int)flags[i]);
	}
}

// Events are delivered on a queue of their own, so no run loop is needed (a test has none, and the program's is the UI's).
static dispatch_queue_t eventQueue(void) {
	static dispatch_queue_t queue;
	static dispatch_once_t once;
	dispatch_once(&once, ^{ queue = dispatch_queue_create("omni.fsevents", DISPATCH_QUEUE_SERIAL); });
	return queue;
}

void *omniWatchStart(long handle, char **paths, int count, double latency) {
	NSMutableArray<NSString *> *list = [NSMutableArray arrayWithCapacity:count];
	for (int i = 0; i < count; i++) {
		[list addObject:[NSString stringWithUTF8String:paths[i]]];
	}
	FSEventStreamContext context = {0, (void *)handle, NULL, NULL, NULL};
	FSEventStreamRef stream = FSEventStreamCreate(NULL, onEvents, &context, (__bridge CFArrayRef)list,
		kFSEventStreamEventIdSinceNow, latency, kFSEventStreamCreateFlagFileEvents | kFSEventStreamCreateFlagNoDefer);
	if (stream == NULL) {
		return NULL;
	}
	FSEventStreamSetDispatchQueue(stream, eventQueue());
	if (!FSEventStreamStart(stream)) {
		FSEventStreamInvalidate(stream);
		FSEventStreamRelease(stream);
		return NULL;
	}
	return stream;
}

void omniWatchStop(void *stream) {
	FSEventStreamRef ref = (FSEventStreamRef)stream;
	FSEventStreamStop(ref);
	FSEventStreamInvalidate(ref);
	FSEventStreamRelease(ref);
}
