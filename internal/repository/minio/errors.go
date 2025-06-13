package minio

import "fmt"

type FileError struct {
	Op  string
	Key string
	Err error
}

func (e *FileError) Error() string {
	return fmt.Sprintf("%s failed for key %s: %v", e.Op, e.Key, e.Err)
}
