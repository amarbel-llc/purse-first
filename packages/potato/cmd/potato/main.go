package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/log"

	"github.com/friedenberg/potato/internal/timer"
	"github.com/friedenberg/potato/internal/zmx"
)

func main() {
	minutes := 5

	if len(os.Args) > 1 {
		n, err := strconv.Atoi(os.Args[1])
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "usage: potato [minutes]\n")
			os.Exit(1)
		}
		minutes = n
	}

	zmx.DetachAll()

	log.Info("Break starting", "minutes", minutes)

	duration := time.Duration(minutes) * time.Minute

	if err := timer.Run(duration); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	log.Info("Break complete!")
	fmt.Print("\a")
}
