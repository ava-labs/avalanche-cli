package publicarchive

import "os"

// IsEmpty returns true if the Downloader is empty and not initialized
func (d Downloader) IsEmpty() bool {
	return d.getter.client == nil
}

// GetDownloadSize returns the size of the download
func (d Downloader) GetDownloadSize() int64 {
	d.getter.mutex.RLock()
	defer d.getter.mutex.RUnlock()
	return d.getter.size
}

func (d Downloader) setDownloadSize(size int64) {
	d.getter.mutex.Lock()
	defer d.getter.mutex.Unlock()
	d.getter.size = size
}

// GetCurrentProgress returns the current download progress
func (d Downloader) GetBytesComplete() int64 {
	d.getter.mutex.RLock()
	defer d.getter.mutex.RUnlock()
	return d.getter.bytesComplete
}

func (d Downloader) setBytesComplete(progress int64) {
	d.getter.mutex.Lock()
	defer d.getter.mutex.Unlock()
	d.getter.bytesComplete = progress
}

func (d Downloader) CleanUp() {
	_ = os.Remove(d.getter.request.Filename)
}
