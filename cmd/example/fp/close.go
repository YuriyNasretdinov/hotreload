package fp

import "os"

// Close is the minimalistic function that wraps os.File
func Close(f *os.File) error {
	return f.Close()
}
