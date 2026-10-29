#ifndef OMNI_CLIPBOARD_H
#define OMNI_CLIPBOARD_H

// The pasteboard's change counter: it goes up whenever anything is copied.
long omniPasteboardCount(void);
// The text on the pasteboard in a malloc'd buffer the caller frees, or NULL when what is there is not text.
char *omniPasteboardString(void);
// 1 when whatever put it there asked for it to be kept out of any history (the nspasteboard.org markers).
int omniPasteboardPrivate(void);
// The name of the application that has the keyboard, malloc'd, or NULL.
char *omniFrontmostName(void);
// The process id of the application that has the keyboard.
int omniClipFrontmostPID(void);
// 1 when a password field has all keyboard input to itself, so no program may type into another.
int omniSecureInput(void);
// 1 when this program has the Accessibility permission; with prompt, macOS asks for it when it has not.
int omniAccessibilityTrusted(int prompt);
// Types Command+V into whatever has the keyboard. 0 when the events were sent.
int omniSendPaste(void);

#endif
