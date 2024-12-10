package publicarchive

// IsEmpty returns true if the Downloader is empty and not initialized
func (d Downloader) IsEmpty() bool {
	return d.getter.client == nil
}

// GetDownloadSize returns the size of the download
func (d Downloader) GetDownloadSize() int64 {
	return d.getter.size
}

// GetCurrentProgress returns the current download progress
func (d Downloader) GetCurrentProgress() int64 {
	return d.getter.bytesComplete
}
