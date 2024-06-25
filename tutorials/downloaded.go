package tutorials

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func makeDownloadedTab(w fyne.Window) fyne.CanvasObject {
	des, err := backend.GetDataFiles()
	if err != nil {
		log.Println("ERROR:", err)
	}
	var datas = [][]string{}
	for _, de := range des {
		fi, err := de.Info()
		if err != nil {
			log.Println("ERROR:", err)
			continue
		}
		// TODO: cross-platform implementation
		if IsHiddenFile(fi.Name()) {
			// skip the hidden file
			continue
		}
		fPath := filepath.Join(backend.GetDataDir(), fi.Name())
		row := []string{fi.Name(), fPath}
		datas = append(datas, row)
	}
	t := widget.NewTableWithHeaders(
		func() (int, int) {
			if len(datas) == 0 {
				return 0, 2
			}
			return len(datas), len(datas[0])
		},
		func() fyne.CanvasObject {
			return container.NewStack(widget.NewLabel("template11"), widget.NewButton("open", func() {}))
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			l := cell.(*fyne.Container).Objects[0].(*widget.Label)
			b := cell.(*fyne.Container).Objects[1].(*widget.Button)
			switch id.Col {
			case 0:
				l.Show()
				b.Hide()
				l.SetText(datas[id.Row][0])
			case 1:
				l.Hide()
				b.Show()
				b.OnTapped = func() {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					// TODO: test in other platforms
					err := openFolder(ctx, datas[id.Row][1])
					if err != nil {
						dialog.ShowError(err, w)
						return
					}
				}
			}
		})
	t.UpdateHeader = func(id widget.TableCellID, cell fyne.CanvasObject) {
		label := cell.(*widget.Label)
		switch id.Col {
		case -1:
			label.SetText(strconv.Itoa(id.Row + 1))
		case 0:
			label.SetText("name")
		case 1:
			label.SetText("action")
		}
	}
	t.SetColumnWidth(0, 750)
	t.SetColumnWidth(1, 50)
	return t
}

func openFolder(ctx context.Context, folderPath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// macOS
		cmd = exec.CommandContext(ctx, "open", folderPath)
	case "windows":
		// Windows
		cmd = exec.CommandContext(ctx, "explorer", folderPath)
	case "linux":
		// Linux
		cmd = exec.CommandContext(ctx, "xdg-open", folderPath)
	default:
		return fmt.Errorf("unsupported platform")
	}
	// Run the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to open folder: %v", err)
	}
	return nil
}
