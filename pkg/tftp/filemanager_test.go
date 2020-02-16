package tftp

import (
	"fmt"
	"strings"
	"testing"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const blockSize = 512

func putThenGet(fm *FileManager, name string, content string) error {
	it, err := fm.Put(name)
	if it == nil || err != nil {
		return err
	}
	for i := 0; i < len(content); i += blockSize {
		block := content[i:min(i+blockSize, len(content))]
		err = it.Write([]byte(block))
		if err != nil {
			return err
		}
	}

	if it, err := fm.Get(name, blockSize); it == nil || err != nil {
		return err
	} else {
		content2 := ""
		for {
			if buf, err := it.Read(); err != nil {
				return err
			} else {
				if buf == nil {
					break
				}
				content2 += string(buf)
			}
		}
		if content != content2 {
			return fmt.Errorf("%v %v", len(content), len(content2))
		}

	}
	return nil
}

func TestFileManager(t *testing.T) {
	str := "0123456789ABCDEF"
	f128 := str + str + str + str + str + str + str + str
	f512 := f128 + f128 + f128 + f128
	f2048 := f512 + f512 + f512 + f512

	fm := FileManager{}
	if err := fm.Init(); err != nil {
		t.Error(err)
	}

	if it, err := fm.Get("f512", 512); it != nil || err == nil || !strings.Contains(err.Error(), "not found") {
		t.Error(it, err)
	}

	if err := putThenGet(&fm, "f512", f512); err != nil {
		t.Error(err)
	}

	if err := putThenGet(&fm, "f2048", f2048); err != nil {
		t.Error(err)
	}
}
