package bitfield

import (
	"math/big"
)

const (
	fieldSize = 1024
)

type Big map[string]Bitfield

func NewBig() Big {
	return make(map[string]Bitfield)
}

func (fields Big) Set(i *big.Int) {
	number, pageNumber := indexOfBig(i, fieldSize)

	page, exists := fields[pageNumber.String()]
	if !exists {
		page = New(fieldSize)
		fields[pageNumber.String()] = page

	}

	page.Set(uint(number.Uint64()))
}

func (fields Big) IsSet(i *big.Int) bool {
	number, pageNumber := indexOfBig(i, fieldSize)

	page, exists := fields[pageNumber.String()]
	if !exists {
		return false
	}

	return page.IsSet(uint(number.Uint64()))
}

func indexOfBig(i *big.Int, size uint) (*big.Int, *big.Int) {
	number := big.NewInt(0)
	pageNumber := big.NewInt(0)

	number.And(i, big.NewInt(0x3FF))
	pageNumber.AndNot(i, big.NewInt(0x3FF))

	return number, pageNumber
}
