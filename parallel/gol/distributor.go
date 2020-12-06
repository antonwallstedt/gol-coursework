package gol

import (
	"fmt"

	"uk.ac.bris.cs/gameoflife/util"
)

const ALIVE = 255
const DEAD = 0

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func makeWorkerHeights(p Params) []int {
	workerHeights := make([]int, p.Threads)
	for i := range workerHeights {
		workerHeights[i] = p.ImageHeight / p.Threads
	}
	remainder := p.ImageHeight % p.Threads
	if remainder != 0 {
		workerHeights[len(workerHeights)-1] += remainder
	}
	return workerHeights
}

func buildWorkerWorld(p Params, world [][]byte, workerHeight, currentThread int) [][]byte {
	workerWorld := makeWorld(workerHeight+2, p.ImageWidth)
	workerWorld[0] = world[(currentThread*workerHeight+p.ImageHeight-1)%p.ImageHeight]
	for y := 1; y <= workerHeight; y++ {
		workerWorld[y] = world[currentThread*workerHeight+y-1]
	}
	workerWorld[workerHeight+1] = world[((currentThread+1)*workerHeight+p.ImageHeight)%p.ImageHeight]
	return workerWorld
}

func worker(p Params, workerHeight int, workerWorld [][]byte, outChan chan<- [][]byte) {
	resultWorld := makeWorld(workerHeight+2, p.ImageWidth)
	for y := 1; y <= workerHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			aliveNeighbours := 0
			for i := -1; i <= 1; i++ {
				for j := -1; j <= 1; j++ {
					if workerWorld[y+i][(x+j+p.ImageWidth)%p.ImageWidth] != 0 {
						aliveNeighbours++
					}
				}
			}
			if workerWorld[y][x] == ALIVE {
				if aliveNeighbours < 2 || aliveNeighbours > 3 {
					resultWorld[y][x] = DEAD
				} else {
					resultWorld[y][x] = ALIVE
				}
			} else {
				if aliveNeighbours == 3 {
					resultWorld[y][x] = ALIVE
				} else {
					resultWorld[y][x] = DEAD
				}
			}
		}
	}
	outChan <- resultWorld
}

func mod(x, m int) int {
	return (x + m) % m
}

func calculateAliveNeighbours(x, y int, world [][]byte) int {
	neighbours := 0
	height := len(world)
	width := len(world[0])
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				if world[mod(y+i, height)][mod(x+j, width)] == ALIVE {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func sum(list []int) int {
	sum := 0
	for _, val := range list {
		sum += val
	}
	return sum
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == ALIVE {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

// Distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)

	world := makeWorld(p.ImageHeight, p.ImageWidth)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			input := <-c.ioInput
			if input == ALIVE {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
			}
			world[y][x] = input
		}
	}

	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.
	turn := 0
	// workerHeights := makeWorkerHeights(p)
	testWorkerHeight := p.ImageHeight / p.Threads
	for turn < p.Turns {
		outChans := make([]chan [][]byte, p.Threads)
		for i := 0; i < p.Threads; i++ {
			outChans[i] = make(chan [][]byte)
			workerWorld := buildWorkerWorld(p, world, testWorkerHeight, i)
			go worker(p, testWorkerHeight, workerWorld, outChans[i])
		}

		for i := 0; i < p.Threads; i++ {
			worldPart := <-outChans[i]
			for y := 0; y < testWorkerHeight; y++ {
				world[i*testWorkerHeight+y] = worldPart[y+1]
			}
		}

		turn++
	}
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(p, world)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
