package tftp

import (
	"fmt"
	"sync"
)

const BlockSize uint = 512

type File struct {
	Filename string
	Data     []byte
}

func NewFile(filename string) File {
	return File{Filename: filename, Data: []byte{}}
}

func (f *File) WriteBlock(n uint, block []byte) (size uint, err error) {
	l := uint(len(f.Data))
	if uint(len(block)) > BlockSize {
		return l, fmt.Errorf("a block can be a maximum of %d bytes long", BlockSize)
	}
	if l/BlockSize != n-1 {
		//only allow adding blocks to the end of the file, never write to the middle
		return l, fmt.Errorf("can't write block to %d", n)
	} else {
		//add block
		f.Data = append(f.Data, block...)
	}
	return uint(len(f.Data)), nil
}

func (f *File) ReadBlock(n uint) (block []byte, ok bool) {
	block = []byte{}
	if f.Data == nil {
		return block, false
	}
	l := uint(len(f.Data))
	start := (n - 1) * BlockSize
	end := start + BlockSize
	if l == 0 && start == 0 {
		return f.Data, true
	} else if l > start {
		if end > l {
			end = l
		}
		return f.Data[start:end], true
	}
	return block, false
}

//FileRepo stores files in memory for transfers that have completed
type FileRepo struct {
	ff map[string]File
	sync.RWMutex
}

func NewFileRepo() *FileRepo {
	return &FileRepo{
		ff: map[string]File{},
	}
}

func (r *FileRepo) Set(file File) bool {
	r.Lock()
	defer r.Unlock()
	r.ff[file.Filename] = file
	return true
}

func (r *FileRepo) Get(filename string) (file File, ok bool) {
	r.RLock()
	defer r.RUnlock()
	file, ok = r.ff[filename]
	return file, ok
}
