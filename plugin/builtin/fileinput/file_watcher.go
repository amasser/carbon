package fileinput

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/bluemedora/bplogagent/entry"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// FileWatcher is a wrapper around `fsnotify` that periodically polls
// to mitigate issues with filesystems that don't support notify events
type FileWatcher struct {
	inode        uint64
	dev          uint64
	path         string
	file         *os.File
	offset       int64
	pollInterval time.Duration
	fileSource   *FileSource
	cancel       context.CancelFunc
	splitFunc    bufio.SplitFunc

	*zap.SugaredLogger
}

func NewFileWatcher(path string, fileSource *FileSource, startFromBeginning bool, splitFunc bufio.SplitFunc, pollInterval time.Duration, logger *zap.SugaredLogger) (*FileWatcher, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	// TODO make this work for windows
	var inode uint64
	var dev uint64
	switch sys := fileInfo.Sys().(type) {
	case *syscall.Stat_t:
		inode = sys.Ino
		dev = uint64(sys.Dev)
	default:
		return nil, fmt.Errorf("cannot use fileinfo of type %T", fileInfo.Sys())
	}

	offset := func() int64 {
		if startFromBeginning {
			return 0
		}
		return fileSize
	}()

	return &FileWatcher{
		inode:         inode,
		dev:           dev,
		path:          path,
		pollInterval:  pollInterval,
		offset:        offset,
		fileSource:    fileSource,
		splitFunc:     splitFunc,
		SugaredLogger: logger.With("path", path),
	}, nil
}

func (w *FileWatcher) Watch(startCtx context.Context) error {
	ctx, cancel := context.WithCancel(startCtx)
	w.cancel = cancel

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		// TODO if falling back to polling, should we set the default lower?
		w.Infow("Failed to create notifying watcher. Falling back to polling only", "error", err)
		watcher = &fsnotify.Watcher{} // create an empty watcher whose channels are just nil
	} else {
		err = watcher.Add(w.path)
		if err != nil {
			w.Infow("Failed to add path to watcher. Falling back to polling only", "error", err)
			watcher = &fsnotify.Watcher{} // create an empty watcher whose channels are just nil
		}
	}

	file, err := os.Open(w.path)
	if err != nil {
		return err
	}
	defer file.Close()
	w.file = file

	w.checkFile(ctx) // Check it once initially for responsive startup

	for {
		// TODO actually test all these cases
		// TODO actually test all these cases on every OS we support
		// TODO actually test all these cases on weird filesystems (NFS, FUSE, etc)

		// TODO reuse the timer? but be careful about draining -- see timer.Reset() docs
		timer := time.NewTimer(w.pollInterval)

		select {
		case <-ctx.Done():
			timer.Stop()
			err := watcher.Close()
			if err != nil {
				return err
			}
		case event, ok := <-watcher.Events:
			timer.Stop()
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Remove > 0 {
				watcher.Close()
				w.fileSource.fileRemoved <- w
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Chmod) > 0 {
				w.checkFile(ctx)
			}
			// ignore chmod and rename (rename is covered by directory create)
		case <-timer.C:
			w.checkFile(ctx)
		case err := <-watcher.Errors:
			timer.Stop()
			return err
		}
	}
}

func (w *FileWatcher) checkFile(ctx context.Context) {
	select {
	case w.fileSource.fileTouched <- w.path:
	case <-ctx.Done():
		return
	}

	fileInfo, err := w.file.Stat()
	if err != nil {
		w.Errorw("Failed to get file info", "error", err) // TODO is this a recoverable error?
		return
	}

	if fileInfo.Size() < w.offset {
		w.Debug("Detected file truncation. Starting from beginning")
		w.offset, err = w.file.Seek(0, 0)
		if err != nil {
			w.Errorw("Failed to seek to file start", "error", err)
			return
		}
		w.readToEnd(ctx)
	} else if fileInfo.Size() > w.offset {
		w.readToEnd(ctx)
	}

	// do nothing if the file hasn't changed size
}

func (w *FileWatcher) readToEnd(ctx context.Context) {
	// TODO seek to last offset?
	scanner := bufio.NewScanner(w.file)
	scanner.Split(w.splitFunc)
	// TODO scanner.Buffer() to set max size

	for {
		select {
		case <-ctx.Done():
			return // Stop reading if closed
		default:
		}

		ok := scanner.Scan()
		if !ok {
			if err := scanner.Err(); err != nil {
				w.Warn("Failed to scan file", "error", err)
			}
			break
		}

		message := scanner.Text()
		entry := &entry.Entry{
			Timestamp: time.Now(),
			Record: map[string]interface{}{
				"message": message,
				"path":    w.path, // TODO use absolute path?
			},
		}

		w.fileSource.Output(entry)

		var err error
		w.offset, err = w.file.Seek(0, 1) // get current file offset
		if err != nil {
			w.Errorw("Failed to get current offset", "error", err)
			return
		}
	}
}

func (w *FileWatcher) Close() {
	if w.cancel != nil {
		w.cancel()
	}
}