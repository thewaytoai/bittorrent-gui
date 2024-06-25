package torrent

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

type Backend struct {
	cli                *torrent.Client
	dataDir            string
	downloadingDir     string
	downloadedDir      string
	torrentsC          chan *torrent.Torrent
	gotInfoC           chan *torrent.Torrent
	torrentDownloadedC chan *torrent.Torrent
	stopC              chan struct{}
}

func NewBackend() (b *Backend, err error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalln(err)
	}
	var mainDir = "bittorrent-gui/"
	dataDir := filepath.Join(homeDir, mainDir, "datas")
	downloadingDir := filepath.Join(homeDir, mainDir, "downloading")
	downloadedDir := filepath.Join(homeDir, mainDir, "downloaded")
	b = &Backend{
		dataDir:        dataDir,
		downloadingDir: downloadingDir,
		downloadedDir:  downloadedDir,
		// These kinds of chan's cap should be 1 to avoid blocking sender
		torrentsC:          make(chan *torrent.Torrent, 1),
		gotInfoC:           make(chan *torrent.Torrent, 1),
		torrentDownloadedC: make(chan *torrent.Torrent, 1),
		stopC:              make(chan struct{}),
	}
	err = os.MkdirAll(dataDir, 0766)
	if err != nil {
		log.Fatalln(err)
	}
	err = os.MkdirAll(downloadingDir, 0766)
	if err != nil {
		log.Fatalln(err)
	}
	err = os.MkdirAll(downloadedDir, 0766)
	if err != nil {
		log.Fatalln(err)
	}
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = dataDir
	b.cli, err = torrent.NewClient(cfg)
	if err != nil {
		log.Fatalln(err)
	}
	return
}

func (b *Backend) Start() {
	go b.run()
}

func (b *Backend) AddBTFile(file io.Reader) (t *torrent.Torrent, err error) {
	metaInfo, err := metainfo.Load(file)
	if err != nil {
		return t, fmt.Errorf("error loading torrent file: %w", err)
	}
	t, err = b.cli.AddTorrent(metaInfo)
	if err != nil {
		return t, fmt.Errorf("adding torrent: %w", err)
	}
	b.torrentsC <- t
	return
}

func (b *Backend) AddTorrentFromFile(file string) (t *torrent.Torrent, err error) {
	t, err = b.cli.AddTorrentFromFile(file)
	if err != nil {
		return t, fmt.Errorf("adding torrent: %w", err)
	}
	b.torrentsC <- t
	return
}

func (b *Backend) AddMegnat(magnetURI string) (t *torrent.Torrent, err error) {
	t, err = b.cli.AddMagnet(magnetURI)
	if err != nil {
		return t, fmt.Errorf("error adding magnet: %w", err)
	}
	fmt.Println("add magnet:", t.Name())
	b.torrentsC <- t
	return
}

func (b *Backend) run() {
	for {
		select {
		case t, ok := <-b.torrentsC:
			if !ok {
				fmt.Println("torrentsChan closed, return")
				return
			}
			go func(fileTorrent *torrent.Torrent) {
				select {
				case <-fileTorrent.GotInfo():
				case <-b.stopC:
					return
				}
				fmt.Println("fileTorrent got info done:", fileTorrent.Name())
				b.gotInfoC <- fileTorrent
				b.waitForAllPiecesOrCancel(fileTorrent, 0, fileTorrent.NumPieces())
			}(t)
		case t := <-b.gotInfoC:
			t.DownloadAll()
			err := b.SaveDownloadingTorrent(t)
			if err != nil {
				log.Println(err)
			}
		case t := <-b.torrentDownloadedC:
			t.Drop()
			err := b.mvTorrentToComplete(t)
			if err != nil {
				log.Println("ERROR:", err)
			}
		// case <-b.cli.WaitAll():
		case <-b.stopC:
			fmt.Println("backend stopped, return")
			return
		}
	}

}

func (b *Backend) Stop() {
	close(b.stopC)
}

func (b *Backend) GetDataFiles() (des []fs.DirEntry, err error) {
	return os.ReadDir(b.dataDir)
}

func (b *Backend) GetDataDir() string {
	return b.dataDir
}

func (b *Backend) GetDownloading() (des []fs.DirEntry, err error) {
	return os.ReadDir(b.downloadingDir)
}

func (b *Backend) GetDownloadingDir() string {
	return b.downloadingDir
}

func (b *Backend) GetDownloaded() (des []fs.DirEntry, err error) {
	return os.ReadDir(b.downloadedDir)
}

func (b *Backend) GetDownloadedDir() string {
	return b.downloadedDir
}

func (b *Backend) GetTorrents() []*torrent.Torrent {
	return b.cli.Torrents()
}

func (b *Backend) SaveDownloadingTorrent(t *torrent.Torrent) (err error) {
	torrentPath := ""
	if t.Name() != "" {
		torrentPath = filepath.Join(b.downloadingDir, t.Name()+".torrent")
	} else {
		torrentPath = filepath.Join(b.downloadingDir, t.InfoHash().HexString()+".torrent")
	}
	_, err = os.Stat(torrentPath)
	if err == nil {
		// TODO: change log level to debug
		// log.Printf("file %s exists, don't need to save bittorrent file.", torrentPath)
		return
	}
	return writeMetainfoToFile(t.Metainfo(), torrentPath)
}

func (b *Backend) mvTorrentToComplete(t *torrent.Torrent) (err error) {
	oldPath, newPath := "", ""
	if t.Name() != "" {
		oldPath = filepath.Join(b.downloadingDir, t.Name()+".torrent")
		newPath = filepath.Join(b.downloadedDir, t.Name()+".torrent")
	} else {
		oldPath = filepath.Join(b.downloadingDir, t.InfoHash().HexString()+".torrent")
		newPath = filepath.Join(b.downloadedDir, t.InfoHash().HexString()+".torrent")
	}
	return os.Rename(oldPath, newPath)
}

func (b *Backend) waitForAllPiecesOrCancel(t *torrent.Torrent, beginIndex, endIndex int) {
	sub := t.SubscribePieceStateChanges()
	defer sub.Close()
	expected := storage.Completion{
		Complete: true,
		Ok:       true,
	}
	pending := make(map[int]struct{})
	for i := beginIndex; i < endIndex; i++ {
		if t.Piece(i).State().Completion != expected {
			pending[i] = struct{}{}
		}
	}
	for {
		if len(pending) == 0 {
			log.Printf("the torrent file: %s download successful!\n", t.Name())
			b.torrentDownloadedC <- t
			return
		}
		select {
		case ev := <-sub.Values:
			if ev.Completion == expected {
				delete(pending, ev.Index)
			}
		case <-t.Closed():
			return
		case <-b.stopC:
			return
		}
	}
}

func writeMetainfoToFile(mi metainfo.MetaInfo, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer f.Close()
	err = mi.Write(f)
	if err != nil {
		return err
	}
	return f.Close()
}
