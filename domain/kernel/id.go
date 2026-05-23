package kernel

import "fmt"

type ID int64

func (id ID) String() string {
	return fmt.Sprintf("%d", id)
}

func (id ID) Int64() int64 {
	return int64(id)
}
