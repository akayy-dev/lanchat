package ui

import (
	"fmt"
	"math/rand"
)

type User struct {
	Name  string
	Color string
}

func generateRandomHexString() string {
	return fmt.Sprintf("#%06X", rand.Intn(0xFFFFFF+1))
}
