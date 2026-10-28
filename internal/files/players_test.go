package files

import (
	"reflect"
	"testing"
)

func TestEachPlayerIsAskedForAMomentTheWayItDocuments(t *testing.T) {
	const file = `C:\Media\Talks\sleep 1.mp3`
	for _, tt := range []struct {
		player  Player
		seconds float64
		want    []string
	}{
		{Player{kind: vlc}, 75, []string{"--start-time=75.0", file}},
		{Player{kind: vlc}, 12.46, []string{"--start-time=12.5", file}},
		{Player{kind: mpv}, 3723, []string{"--start=3723.0", file}},
		{Player{kind: mpc}, 75, []string{file, "/start", "75000"}},
		{Player{kind: mpc}, 0.25, []string{file, "/start", "250"}},
		{Player{kind: vlc}, -4, []string{"--start-time=0.0", file}},
	} {
		if got := tt.player.args(file, tt.seconds); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("kind %d at %v: args = %q, want %q", tt.player.kind, tt.seconds, got, tt.want)
		}
	}
}
