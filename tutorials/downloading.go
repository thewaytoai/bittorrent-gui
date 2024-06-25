package tutorials

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func makeDownloadingTab(w fyne.Window) fyne.CanvasObject {
	table := widget.NewTableWithHeaders(
		func() (int, int) {
			return tableData.Length(), 4
		},
		func() fyne.CanvasObject {
			return container.NewStack(widget.NewLabel("name"), widget.NewLabel("speed"), widget.NewProgressBar(), widget.NewButton("remove", func() {}))
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			item, err := tableData.GetValue(id.Row)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			t, ok := item.(*torrentWithProgress)
			if !ok {
				dialog.ShowError(errors.New("type transfer error"), w)
				return
			}

			nameLabel := cell.(*fyne.Container).Objects[0].(*widget.Label)
			speedLabel := cell.(*fyne.Container).Objects[1].(*widget.Label)
			p := cell.(*fyne.Container).Objects[2].(*widget.ProgressBar)
			b := cell.(*fyne.Container).Objects[3].(*widget.Button)
			nameLabel.Show()
			speedLabel.Hide()
			p.Hide()
			b.Hide()
			switch id.Col {
			case 0:
				nameLabel.SetText(t.Name())
			case 1:
				speedLabel.Show()
				nameLabel.Hide()
				speedLabel.Bind(t.downloadSpeed)
			case 2:
				p.Show()
				nameLabel.Hide()
				p.Bind(t.progress)
			case 3:
				b.Show()
				nameLabel.Hide()
				b.OnTapped = func() {
					dialog.ShowConfirm("Are you sure to remove this file?", t.Name(), func(b bool) {
						if b {
							t.Drop()
							torrentPath := filepath.Join(backend.GetDownloadingDir(), t.Name()+".torrent")
							err := os.RemoveAll(torrentPath)
							if err != nil {
								dialog.ShowError(err, w)
								return
							}
							fPath := filepath.Join(backend.GetDataDir(), t.Name())
							err = os.RemoveAll(fPath)
							if err != nil {
								dialog.ShowError(err, w)
								return
							}
						}
					}, w)
				}
			}
		})
	table.UpdateHeader = func(id widget.TableCellID, cell fyne.CanvasObject) {
		label := cell.(*widget.Label)
		switch id.Col {
		case -1:
			label.SetText(strconv.Itoa(id.Row + 1))
		case 0:
			label.SetText("name")
		case 1:
			label.SetText("speed")
		case 2:
			label.SetText("progress")
		case 3:
			label.SetText("action")
		}
	}

	table.SetColumnWidth(0, 430)
	table.SetColumnWidth(2, 250)
	magnetInput := widget.NewEntry()
	magnetInput.SetPlaceHolder("please paste your magnet uri here")
	downloadPart := container.NewVBox(
		layout.NewSpacer(),

		container.NewGridWithColumns(2, magnetInput,
			widget.NewButton("commit magnet uri", func() {
				t, err := backend.AddMegnat(magnetInput.Text)
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				magnetInput.SetText("")
				tableData.Append(&torrentWithProgress{t, binding.NewFloat(), binding.NewString()})
			})),

		layout.NewSpacer(),
		widget.NewButton("open a .torrent file to download", func() {
			fd := dialog.NewFileOpen(func(f fyne.URIReadCloser, _ error) {
				if f == nil {
					return
				}
				t, err := backend.AddBTFile(f)
				if err != nil {
					dialog.ShowError(err, w)
				}
				tableData.Append(&torrentWithProgress{t, binding.NewFloat(), binding.NewString()})
			}, w)
			fd.SetFilter(storage.NewExtensionFileFilter([]string{".torrent"}))
			fd.Show()
		}),
		layout.NewSpacer(),
	)
	return container.NewGridWithRows(2, table, downloadPart)
}
