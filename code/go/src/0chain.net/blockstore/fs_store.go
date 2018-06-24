package blockstore

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"path/filepath"

	"0chain.net/block"
	"0chain.net/datastore"
)

/*FSBlockStore - a block store implementation using file system */
type FSBlockStore struct {
	RootDirectory string
}

const (
	DIR_ROUND_RANGE = 10000000
)

/*SetupFSBlockStore - Setup a file system based block storage */
func SetupFSBlockStore(rootDir string) {
	Store = &FSBlockStore{RootDirectory: rootDir}
}

func (fbs *FSBlockStore) getFileName(hash string, round int64) string {
	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s%s%v", fbs.RootDirectory, string(os.PathSeparator), int64(round/DIR_ROUND_RANGE))
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), hash[3*i:3*i+3])
	}
	fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), hash[9:])
	fmt.Fprintf(&dir, ".dat.zlib")
	return dir.String()
}

/*GetFileName - given a block, get the file name it maps to */
func (fbs *FSBlockStore) GetFileName(b *block.Block) string {
	return fbs.getFileName(b.Hash, b.Round)
}

/*Write - write the block to the file system */
func (fbs *FSBlockStore) Write(b *block.Block) error {
	fileName := fbs.GetFileName(b)
	dir := filepath.Dir(fileName)
	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	bf := bufio.NewWriterSize(f, 64*1024)
	w, _ := zlib.NewWriterLevel(bf, zlib.BestCompression)
	datastore.WriteJSON(w, b)
	w.Close()
	bf.Flush()
	f.Close()
	return nil
}

/*Read - read the block from the file system */
func (fbs *FSBlockStore) Read(hash string, round int64) (*block.Block, error) {
	fileName := fbs.getFileName(hash, round)
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := zlib.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var b block.Block
	err = datastore.ReadJSON(r, &b)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
