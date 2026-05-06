package models

import "time"

type Block struct {
	Height int
	Time   time.Time
	Size   int
}
