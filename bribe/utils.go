package bribe

import (
	"math/rand"

	"time"
)

// 乱数は先攻後攻の決定や選択したセルに印を置かれるかなどの決定に使用
func createLocalRandGenerator() *rand.Rand {
	source := rand.NewSource(time.Now().UnixNano())
	return rand.New(source)
}
