package key

import (
	"fmt"
	"time"
)

func chatKey() string{
	return fmt.Sprintf("chat#%d", time.Now().UnixMilli())
}