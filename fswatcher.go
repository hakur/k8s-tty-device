package main

import (
	fsnotify "gopkg.in/fsnotify.v1"
)

// newFsWatcher : create a new file watch by fsnotify,when it delete,will receive a message
func newFileWatcher(file string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return watcher, nil
}
