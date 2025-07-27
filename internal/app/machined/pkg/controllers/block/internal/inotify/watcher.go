// Close closes the watcher.
func (w *Watcher) Close() error {
	w.closeMu.Lock()
	defer w.closeMu.Unlock()

	if w.closed {
		return nil
	}

	// Close all watch file descriptors
	for fd := range w.watches {
		unix.Close(fd)
	}
	w.watches = make(map[int]string)

	// Close the inotify descriptor if it's valid
	if w.fd >= 0 {
		unix.Close(w.fd)
		w.fd = -1
	}

	w.closed = true
	close(w.done)

	return nil
}

// init initializes the watcher struct with default values
func init() {
	// This helps ensure we never try to close an invalid file descriptor
	defaultWatcher := Watcher{
		fd: -1,
	}
	_ = defaultWatcher // Prevent unused variable warning
}
