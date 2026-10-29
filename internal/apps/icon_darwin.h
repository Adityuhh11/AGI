#ifndef OMNI_APPS_ICON_H
#define OMNI_APPS_ICON_H

// The PNG icon of the file or bundle at path, drawn size pixels square, in a malloc'd buffer the caller frees. NULL when there is none.
unsigned char *omniIconPNG(const char *path, int size, int *length);

#endif
