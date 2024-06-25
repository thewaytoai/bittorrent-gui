package tutorials

import (
	"bittorrent-gui/torrent"
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2/data/binding"
	orignalT "github.com/anacrolix/torrent"
	"github.com/dustin/go-humanize"
)

var backend *torrent.Backend
var tableData = binding.NewUntypedList()

type torrentWithProgress struct {
	*orignalT.Torrent
	progress      binding.Float
	downloadSpeed binding.String
}

func init() {
	var err error
	backend, err = torrent.NewBackend()
	if err != nil {
		log.Fatalln(err)
	}
	backend.Start()
	// load unfinished torrent and start to download no matter whether user click the 'downloading'
	des, err := backend.GetDownloading()
	if err != nil {
		log.Println("ERROR:", err)
	}
	for _, de := range des {
		if de.IsDir() {
			continue
		}
		if !strings.HasSuffix(de.Name(), ".torrent") {
			continue
		}
		fi, err := de.Info()
		if err != nil {
			log.Println("ERROR:", err)
			continue
		}
		fPath := filepath.Join(backend.GetDownloadingDir(), fi.Name())
		_, err = backend.AddTorrentFromFile(fPath)
		if err != nil {
			log.Println("ERROR:", err)
		}
	}
	// init downloading data
	torrents := backend.GetTorrents()
	var allLastStats = make(map[*torrentWithProgress]orignalT.TorrentStats, len(torrents))
	for _, tor := range torrents {
		var twp = &torrentWithProgress{
			tor,
			binding.NewFloat(),
			binding.NewString(),
		}
		if tor.Info() != nil {
			var value = float64(tor.Stats().PiecesComplete) / float64(tor.NumPieces())
			twp.progress.Set(value)
			allLastStats[twp] = twp.Stats()
		}
		tableData.Append(twp)
	}
	ctx := context.Background()
	go func() {
		interval := 1 * time.Second
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				allDatas, err := tableData.Get()
				if err != nil {
					log.Println(err)
					continue
				}
				var newData []interface{}
				for _, v := range allDatas {
					t, ok := v.(*torrentWithProgress)
					if !ok {
						log.Println("can not convert to type: torrentWithProgress")
						continue
					}
					// if t is remove by user's click event
					select {
					case <-t.Closed():
						// continue means that the t won't be appended to the new table data, it equals to the DELETE
						continue
					default:
					}

					if t.Complete.Bool() {
						// continue means that the t won't be appended to the new table data, it equals to the DELETE
						continue
					}
					if t.Info() != nil {
						// TODO: progrss should be calculate by stats
						// set progress
						t.progress.Set(float64(t.Stats().PiecesComplete) / float64(t.NumPieces()))
						// set speed
						lastStats := allLastStats[t]
						stats := t.Stats()
						byteRate := int64(time.Second)
						byteRate *= stats.BytesReadUsefulData.Int64() - lastStats.BytesReadUsefulData.Int64()
						byteRate /= int64(interval)
						allLastStats[t] = t.Stats()
						t.downloadSpeed.Set(humanize.Bytes(uint64(byteRate)) + "/s")
					} else {
						t.downloadSpeed.Set("waiting for available data")
					}
					newData = append(newData, t)
				}
				err = tableData.Set(newData)
				if err != nil {
					log.Println(err)
					continue
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
