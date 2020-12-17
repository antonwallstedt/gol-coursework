package main

import (
	"fmt"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

const (
	turns       = 100
	imageHeight = 5120
	imageWidth  = 5120
)

func Benchmark(b *testing.B) {
	params := gol.Params{Turns: turns, ImageHeight: imageHeight, ImageWidth: imageWidth}
	for threads := 1; threads <= 8; threads++ {
		params.Threads = threads
		testName := fmt.Sprintf("%dx%dx%d-%d", params.ImageHeight, params.ImageWidth, params.Turns, params.Threads)
		b.Run(testName, func(b *testing.B) {
			b.StartTimer()
			events := make(chan gol.Event)
			gol.Run(params, events, nil)
			for event := range events {
				switch event.(type) {
				case gol.FinalTurnComplete:
					break
				}
			}
			b.StopTimer()
		})
	}
}
