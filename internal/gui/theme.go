package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// folderIcons draws the folders of the list as folders, rather than as the
// arrow a tree comes with: a shut one where the commands under it are out of
// sight, and an open one where they are not.
//
// A tree asks the theme for those two icons by name, so this is where they are
// answered. The list is the only thing in the window with folders in it, and
// nothing else is touched.
type folderIcons struct {
	fyne.Theme
}

func (t folderIcons) Icon(name fyne.ThemeIconName) fyne.Resource {
	switch name {
	case theme.IconNameNavigateNext:
		name = theme.IconNameFolder
	case theme.IconNameMoveDown:
		name = theme.IconNameFolderOpen
	}
	return t.Theme.Icon(name)
}
