package muxfs

import (
	"io/fs"
	"time"
)

type fileInfo struct {
	mode    fs.FileMode
	modTime time.Time
	name    string
	size    int64
}

func (i fileInfo) IsDir() bool                { return i.mode.IsDir() }
func (i fileInfo) ModTime() time.Time         { return i.modTime }
func (i fileInfo) Mode() fs.FileMode          { return i.mode }
func (i fileInfo) Type() fs.FileMode          { return i.mode.Type() }
func (i fileInfo) Name() string               { return i.name }
func (i fileInfo) Size() int64                { return i.size }
func (i fileInfo) Info() (fs.FileInfo, error) { return i, nil }
func (fileInfo) Sys() any                     { return nil }

var _ fs.FileInfo = fileInfo{}
var _ fs.DirEntry = fileInfo{}
