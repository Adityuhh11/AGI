package watcher

import "testing"

func TestRelevant(t *testing.T) {
	tests := []struct {
		rel  string
		want bool
	}{
		{`notes.md`, true},
		{`Documents\Prob and stats\Week 06.pdf`, true},
		{`src\app\main.go`, true},
		{`folder moved here`, true}, // a folder appearing or vanishing matters too
		{`Pictures\Screenshots\Screenshot 2026-10-16.png`, true},
		{``, false},
		{`.git\index`, false},
		{`project\.git\objects\ab\cdef`, false},
		{`project\.venv\Lib\site.py`, false},
		{`project\node_modules\react\index.js`, false},
		{`project\NODE_MODULES\react\index.js`, false},
		{`project\__pycache__\a.cpython-313.pyc`, false},
		{`project\venv\Scripts\python.exe`, false},
		{`app\.next\cache\x.js`, false},
		{`.hidden.txt`, false},
		{`Unconfirmed 123456.crdownload`, false},
		{`report.docx.tmp`, false},
		{`~$report.docx`, false},
		{`main.go~`, false},
		{`.main.go.swp`, false},
		{`a/b/c.txt`, true}, // forward slashes are accepted
		{`a/.git/HEAD`, false},
	}
	for _, tt := range tests {
		if got := relevant(tt.rel); got != tt.want {
			t.Errorf("relevant(%q) = %v, want %v", tt.rel, got, tt.want)
		}
	}
}
