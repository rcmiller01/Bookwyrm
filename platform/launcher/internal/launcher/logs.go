package launcher

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

const (
	defaultRotateSize = int64(10 * 1024 * 1024)
	defaultRotateKeep = 5
)

func NewLauncherLogger(path string) (*log.Logger, io.Closer, error) {
	file, err := openRotatingFile(path, defaultRotateSize, defaultRotateKeep)
	if err != nil {
		return nil, nil, err
	}
	mw := io.MultiWriter(os.Stdout, file)
	return log.New(mw, "", log.LstdFlags), file, nil
}

func openRotatingFile(path string, maxBytes int64, keep int) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if info, err := os.Stat(path); err == nil && info.Size() >= maxBytes {
		for i := keep; i >= 1; i-- {
			src := path
			if i > 1 {
				src = path + "." + itoa(i-1)
			}
			dst := path + "." + itoa(i)
			_, srcErr := os.Stat(src)
			if srcErr == nil {
				_ = os.Rename(src, dst)
			}
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	buf := make([]byte, 0, 8)
	for value > 0 {
		buf = append([]byte{byte('0' + (value % 10))}, buf...)
		value /= 10
	}
	return string(buf)
}
