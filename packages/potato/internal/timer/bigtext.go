package timer

import (
	"fmt"
	"time"

	figure "github.com/common-nighthawk/go-figure"
)

func renderBigTime(remaining time.Duration) string {
	minutes := int(remaining.Minutes())
	seconds := int(remaining.Seconds()) % 60
	text := fmt.Sprintf("%02d:%02d", minutes, seconds)
	fig := figure.NewFigure(text, "banner3", true)
	return fig.String()
}
