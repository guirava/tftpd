package tftp

import (
	"bytes"
	"fmt"
)

type FileManager struct {
	files map[string][]byte
}

type FileIterator struct {
	fileManager *FileManager
	filename    string
	blockSize   int
	position    int
}

func (f *FileManager) Init() (err error) {
	f.files = make(map[string][]byte)
	return
}

func (f *FileManager) DeInit() (err error) {
	return
}
func (fm *FileManager) Exists(filename string) bool {
	_, ok := fm.files[filename]
	return ok
}

func (f *FileManager) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{")
	count := 0
	for fileName, fileContent := range f.files {
		if count != 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(fmt.Sprintf("\"%s\":%d", fileName, len(fileContent)))
		count++
	}
	buffer.WriteString("}")
	return buffer.Bytes(), nil
}

func (fm *FileManager) Get(filename string, readSize int) (file *FileIterator, err error) {
	if _, ok := fm.files[filename]; ok {
		return &FileIterator{fm, filename, readSize, 0}, nil
	}
	return nil, fmt.Errorf("%v not found", filename)

}

func (fm *FileManager) Put(filename string) (file *FileIterator, err error) {
	// Fail if the file already exists at the server, we do not handle overwrites:
	if _, ok := fm.files[filename]; ok {
		return nil, fmt.Errorf("%v already exists", filename)
	}
	return &FileIterator{fm, filename, -1, -1}, nil

}
func (it *FileIterator) Read() ([]byte, error) {
	if file, ok := it.fileManager.files[it.filename]; ok {
		start, end := it.position, it.position+it.blockSize
		if start >= len(file) {
			return nil, nil
		}
		if end > len(file) {
			end = len(file)
		}
		it.position = end
		return file[start:end], nil
	}
	// file was there originally since we got an iterator on it, but
	// now it's gone
	return nil, fmt.Errorf("{it.filename} access violation")
}

func (it *FileIterator) Write(buf []byte) error {
	//if file, ok := it.fileManager.files[it.filename]; ok {
	it.fileManager.files[it.filename] = append(it.fileManager.files[it.filename], buf...)
	//}

	return nil
}
