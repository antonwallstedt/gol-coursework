package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

const (
	turns       = 250
	imageHeight = 512
	imageWidth  = 512
)

func BenchmarkSerial(b *testing.B) {
	params := gol.Params{Turns: turns, ImageWidth: imageWidth, ImageHeight: imageHeight, Threads: 1}
	testName := fmt.Sprintf("%dx%dx%d", params.ImageHeight, params.ImageWidth, params.Turns)
	os.Stdout = nil
	for i := 0; i < b.N; i++ {
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
