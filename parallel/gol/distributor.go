package gol

import (
	"fmt"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

const ALIVE = 255
const DEAD = 0

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFileName chan<- string
	ioInput    <-chan uint8
	ioOutput   chan<- uint8
}

// func buildWorkerWorld(world [][]byte, imageHeight, imageWidth, currentThreads int) [][]byte {
// 	workerWorld := make([][]byte, imageHeight)
// 	for r := range workerWorld {
// 		workerWorld[r] = make([]byte, imageWidth)
// 	}

// 	return workerWorld
// }
//This buildworker function is used to create a world for a worker in each thread.(Example for running with thread 2, you need to divide the work for the worker in to two part)
func buildWorkerWorld(world [][]byte, workerHeight, imageHeight, imageWidth, currentThreads, Threads int) [][]byte {
	workerWorld := make([][]byte, workerHeight+2)
	for j := range workerWorld {
		workerWorld[j] = make([]byte, imageWidth)
	}

	if currentThreads == Threads-1 {
		workerHeight1 := workerHeight - imageHeight%Threads
		for x := 0; x < imageWidth; x++ {
			workerWorld[0][x] = world[(currentThreads*workerHeight1+imageHeight-1)%imageHeight][x]
		}
		for y := 1; y <= workerHeight; y++ {
			for x := 0; x < imageWidth; x++ {
				workerWorld[y][x] = world[currentThreads*workerHeight1+y-1][x]
			}
		}
		for x := 0; x < imageWidth; x++ {
			workerWorld[workerHeight+1][x] = world[0][x]
		}
	} else {
		for x := 0; x < imageWidth; x++ {
			workerWorld[0][x] = world[(currentThreads*workerHeight+imageHeight-1)%imageHeight][x]
		}
		for y := 1; y <= workerHeight; y++ {
			for x := 0; x < imageWidth; x++ {
				workerWorld[y][x] = world[currentThreads*workerHeight+y-1][x]
			}
		}
		for x := 0; x < imageWidth; x++ {
			workerWorld[workerHeight+1][x] = world[((currentThreads+1)*workerHeight+imageHeight)%imageHeight][x]
		}

	}

	return workerWorld
}

// Function to find out a alive neighbor of the cell
func mod(x, m int) int {
	return (x + m) % m
}

func calculateNeighbours(p Params, x, y int, world [][]byte) int {
	neighbours := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				if world[mod(y+i, p.ImageHeight)][mod(x+j, p.ImageWidth)] == ALIVE {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

//Worker is the functino that used to calculate the logic of the program and giving each byte of newWorld to distributor for finalComplete turn channel.
func worker(p Params, workerChan chan byte, imageHeight int, imageWidth int, outChan chan byte, Thread, currentThread int) {

	world := make([][]byte, imageHeight+2)
	for i := range world {
		world[i] = make([]byte, imageWidth)
	}
	for y := 0; y < imageHeight+2; y++ {
		for x := 0; x < imageWidth; x++ {
			world[y][x] = <-workerChan
		}
	}

	newWorld := make([][]byte, imageHeight+2)
	for i := range world {
		newWorld[i] = make([]byte, imageWidth)
	}
	//we don't need to care about the first row, cause we need to ignore every first role.
	for y := 1; y <= imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			var neighboursAlive = 0
			neighboursAlive = calculateNeighbours(p, x, y, world)
			if world[y][x] == ALIVE {
				if neighboursAlive == 2 || neighboursAlive == 3 {
					newWorld[y][x] = ALIVE
				} else {
					newWorld[y][x] = DEAD
				}
			} else {
				if neighboursAlive == 3 {
					newWorld[y][x] = ALIVE
				} else {
					newWorld[y][x] = DEAD
				}

			}
		}
	}
	//Here is where we ignore the first and the last row.
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			outChan <- newWorld[y+1][x]
		}
	}

}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	var FinalTurnComplete FinalTurnComplete
	var mutex sync.Mutex

	c.ioCommand <- ioInput
	c.ioFileName <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)

	var listCell []util.Cell

	// TODO: Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}
	newWorld := make([][]byte, p.ImageHeight)
	for i := range world {
		newWorld[i] = make([]byte, p.ImageWidth)
	}

	// TODO: For all initially alive cells send a CellFlipped Event.
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			input := <-c.ioInput
			world[y][x] = input

		}
	}

	turn := 0

	for turn < p.Turns {
		ticker := time.NewTicker(2 * time.Second)
		done := make(chan bool)
		go func() {
			select {

			case <-ticker.C:
				var aliveCell int
				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						if world[y][x] == ALIVE {
							aliveCell++
						}
					}
				}
				mutex.Lock()
				c.events <- AliveCellsCount{turn, aliveCell}
				mutex.Unlock()

			case <-done:
				return
			}

		}()

		var workerHeight int
		outChan := make([]chan byte, p.Threads)
		workerHeight = p.ImageHeight / p.Threads
		// modOfWorkerHeight := p.ImageWidth % p.Threads
		for i := 0; i < p.Threads; i++ {
			outChan[i] = make(chan byte)
			workerChan := make(chan byte)

			if i == p.Threads-1 {
				workerHeight1 := (p.ImageHeight / p.Threads) + (p.ImageHeight % p.Threads)
				workerWorld := buildWorkerWorld(world, workerHeight1, p.ImageHeight, p.ImageWidth, i, p.Threads)
				go worker(p, workerChan, workerHeight1, p.ImageWidth, outChan[i], p.Threads, i)
				for y := 0; y < workerHeight1+2; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						workerChan <- workerWorld[y][x]
					}
				}
				for y := 0; y < workerHeight1; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						newWorld[i*workerHeight+y][x] = <-outChan[i]
					}
				}
			} else {
				workerWorld := buildWorkerWorld(world, workerHeight, p.ImageHeight, p.ImageWidth, i, p.Threads)
				go worker(p, workerChan, workerHeight, p.ImageWidth, outChan[i], p.Threads, i)
				for y := 0; y < workerHeight+2; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						workerChan <- workerWorld[y][x]
					}
				}
				for y := 0; y < workerHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						newWorld[i*workerHeight+y][x] = <-outChan[i]

					}
				}
			}
		}

		x := world
		world = newWorld
		newWorld = x
		turn++
	}
	//Do an ioCommand output to output the pgm file.

	c.ioCommand <- ioOutput
	c.ioFileName <- fmt.Sprintf("%vx%vx%v", p.ImageHeight, p.ImageWidth, p.Turns)

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			//Give each byte of world to writePGM file
			c.ioOutput <- world[y][x]
			if world[y][x] == ALIVE {
				listCell = append(listCell, util.Cell{Y: y, X: x})
			}
		}
	}

	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	FinalTurnComplete.Alive = listCell
	FinalTurnComplete.CompletedTurns = turn
	c.events <- FinalTurnComplete
	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	mutex.Lock()
	close(c.events)
	mutex.Lock()

}
func printBoard(d distributorChannels, p Params, world [][]byte, turn int) {

	d.ioCommand <- ioOutput
	d.ioFileName <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)

}
