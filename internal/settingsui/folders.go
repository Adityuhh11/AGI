package settingsui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"omni/internal/config"
)

// The folders the setup guide offers to search and leave out. The File search page keeps its own one-at-a-time
// "Add folder" and "Remove"; both change the same list of roots.

// FolderChoice is a folder the guide offers: where it is and whether it starts ticked.
type FolderChoice struct {
	Name, Path string
	Checked    bool
	// Blocked says the system has not let Omni read the folder, which only matters for one that is ticked (a Mac asks for each protected folder).
	Blocked bool
}

// suggested are the user folders worth offering, in the order the guide lists them. Downloads is offered but starts
// unticked: it holds installers and one-off files more than what a person looks for later.
var suggested = []struct {
	name    string
	checked bool
}{
	{"Documents", true}, {"Pictures", true}, {"Videos", true}, {"Desktop", true}, {"Downloads", false},
}

// SuggestedFolders lists the user folders that exist, where Windows keeps them now (OneDrive may have moved Documents,
// Desktop and Pictures), then any other folder already being searched. With no folders yet the usual ones start ticked;
// otherwise a folder is ticked exactly when it is being searched.
func (s *Service) SuggestedFolders() []FolderChoice {
	out := suggestFolders(s.known, func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}, s.o.Store.Get().Roots)
	chosen := map[string]bool{}
	for _, root := range s.o.Store.Get().Roots {
		chosen[config.SourceID(root)] = true
	}
	for i := range out {
		// Only a folder already chosen is read: a first-run tick is a suggestion, and reading it would make a Mac ask about it before the person has agreed.
		if chosen[config.SourceID(out[i].Path)] {
			out[i].Blocked = s.isBlocked(out[i].Path)
		}
	}
	return out
}

// suggestFolders is SuggestedFolders with the Windows lookup and the disk passed in.
func suggestFolders(known func(name string) (string, error), isDir func(path string) bool, roots []string) []FolderChoice {
	searched := map[string]bool{}
	for _, root := range roots {
		searched[config.SourceID(root)] = true
	}
	var out []FolderChoice
	listed := map[string]bool{}
	for _, f := range suggested {
		path, err := known(f.name)
		if err != nil || !isDir(path) {
			continue
		}
		id := config.SourceID(path)
		checked := searched[id]
		if len(roots) == 0 {
			checked = f.checked
		}
		out = append(out, FolderChoice{Name: f.name, Path: filepath.Clean(path), Checked: checked})
		listed[id] = true
	}
	for _, root := range roots {
		if id := config.SourceID(root); !listed[id] {
			out = append(out, FolderChoice{Name: filepath.Base(root), Path: root, Checked: true})
			listed[id] = true
		}
	}
	return out
}

// SetRoots makes these the folders that are searched. A folder left out inside a folder that is no longer searched is
// forgotten with it.
func (s *Service) SetRoots(roots []string) string {
	cleaned := make([]string, 0, len(roots))
	for _, root := range roots {
		cleaned = append(cleaned, filepath.Clean(root))
	}
	var added []string
	for _, root := range cleaned {
		if !slices.Contains(s.o.Store.Get().Roots, root) && !slices.Contains(added, root) {
			added = append(added, root)
		}
	}
	msg := reason(s.o.Store.Update(func(c *config.Config) {
		c.Roots = cleaned
		c.Exclude = slices.DeleteFunc(slices.Clone(c.Exclude), func(folder string) bool {
			for _, root := range cleaned {
				if len(config.ExcludedUnder(root, []string{folder})) > 0 {
					return false
				}
			}
			return true
		})
	}))
	if msg == "" { // a folder that was not added is not asked about
		s.ask(added)
	}
	return msg
}

// LeaveOut asks for a folder inside root and leaves it out of the index; what it held is dropped at the next sync.
// Cancelling the picker changes nothing.
func (s *Service) LeaveOut(root string) string {
	folder, err := s.o.PickFolder()
	if err != nil {
		return fmt.Sprintf("the folder picker failed: %v", err)
	}
	if folder == "" {
		return ""
	}
	folder = filepath.Clean(folder)
	if len(config.ExcludedUnder(root, []string{folder})) == 0 {
		return fmt.Sprintf("%s is not inside %s", folder, root)
	}
	return reason(s.o.Store.Update(func(c *config.Config) {
		if !slices.Contains(c.Exclude, folder) {
			c.Exclude = append(c.Exclude, folder)
		}
	}))
}

// StopLeavingOut puts a folder back into the search.
func (s *Service) StopLeavingOut(folder string) string {
	return reason(s.o.Store.Update(func(c *config.Config) {
		c.Exclude = slices.DeleteFunc(slices.Clone(c.Exclude), func(f string) bool { return f == folder })
	}))
}
