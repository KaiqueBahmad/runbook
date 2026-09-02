package gui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// TestFolderIcons is that a folder in the list looks like one, and like the
// right one: a tree asks for these two icons to draw a branch, and what comes
// back is what a person sees beside the name.
func TestFolderIcons(t *testing.T) {
	under := theme.DefaultTheme()
	icons := folderIcons{under}

	tests := []struct {
		name string
		of   fyne.ThemeIconName
		want fyne.ThemeIconName
	}{
		{"a folder whose commands are out of sight", theme.IconNameNavigateNext, theme.IconNameFolder},
		{"a folder whose commands are shown", theme.IconNameMoveDown, theme.IconNameFolderOpen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := icons.Icon(tt.of)
			if want := under.Icon(tt.want); got.Name() != want.Name() {
				t.Errorf("Icon(%q) = %q, want %q", tt.of, got.Name(), want.Name())
			}
		})
	}

	t.Run("everything else is the theme underneath", func(t *testing.T) {
		for _, name := range []fyne.ThemeIconName{theme.IconNameMediaPlay, theme.IconNameMediaStop, theme.IconNameViewRefresh} {
			if got, want := icons.Icon(name), under.Icon(name); got.Name() != want.Name() {
				t.Errorf("Icon(%q) = %q, want %q", name, got.Name(), want.Name())
			}
		}
	})
}
