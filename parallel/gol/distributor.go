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
	ioInput    <-chan uint8
	ioOutput   chan<- uint8
}

type workerWorld struct {
	data [][]byte
}

/* mod : Calculates the remainder of a given number x when divided by m.
Doesn't allow negative numbers so m is added to x before computing mod. */
func mod(x, m int) int {
	return (x + m) % m
}

/* calculateNeighbours : Counts the number of alive neighbours around a given cell.
Does it in a closed domain, i.e. the top-most pixels are connected to the bottom,
and left-most pixels are connected to the right-most pixels and vice versa. */
func calculateNeighbours(x, y int, world [][]byte) int {
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

/* calculateNextState : Completes one whole evolution of the game and returns the new state */
func calculateNextState(p Params, world [][]byte) [][]byte {
	height := len(world)
	width := p.ImageWidth
	newWorld := makeWorld(height, width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			neighbours := calculateNeighbours(x, y, world)
			if world[y][x] == ALIVE {
				if neighbours == 2 || neighbours == 3 {
					newWorld[y][x] = ALIVE
				} else {
					newWorld[y][x] = DEAD
				}
			} else {
				if neighbours == 3 {
					newWorld[y][x] = ALIVE
				} else {
					newWorld[y][x] = DEAD
				}
			}
		}
	}
	return newWorld
}

/* calculateAliveCells : Locates alive cells in a given world and returns a slice containing coordinates to those cells */
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

/* makeWorld : Creates a 2D world from given height and width */
func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func makeWorkerWorld(p Params, workerHeight int, world [][]byte, currentThread int) [][]byte {
	workerWorld := makeWorld(workerHeight+2, p.ImageWidth)
	workerWorld[0] = world[(currentThread*workerHeight+p.ImageHeight-1)%p.ImageHeight]
	for y := 1; y <= workerHeight; y++ {
		workerWorld[y] = world[currentThread*workerHeight+y-1]
	}
	workerWorld[workerHeight] = world[((currentThread+1)*workerHeight+p.ImageHeight)%p.ImageHeight]
	return workerWorld
}

func worker(p Params, workerHeight int, workerWorld [][]byte, out chan<- [][]byte) {
	for _, row := range workerWorld {
		fmt.Println(row)
	}
	workerWorld = calculateNextState(p, workerWorld)
	workerWorldPart := workerWorld[1:(workerHeight - 1)]

	out <- workerWorldPart
}

/* distributor : Divides the work between workers and interacts with other goroutines. */
func distributor(p Params, c distributorChannels) {

	// Request input from and send filename to IO
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)

	// TODO: For all initially alive cells send a CellFlipped Event.
	world := makeWorld(p.ImageHeight, p.ImageWidth)

	// Load in cells into our new world from a given file
	for y := range world {
		for x := range world {
			world[y][x] = <-c.ioInput
		}
	}

	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.
	defaultHeight := p.ImageHeight / p.Threads
	workerHeights := make([]int, p.Threads)
	for i := range workerHeights {
		workerHeights[i] = defaultHeight
	}

	remainder := p.ImageHeight % p.Threads
	if remainder != 0 {
		workerHeights[len(workerHeights)-1] += remainder
	}

	outChans := make([]chan [][]byte, p.Threads)
	for i := range outChans {
		outChans[i] = make(chan [][]byte)
	}

	workerWorld := makeWorld(p.ImageHeight, p.ImageWidth)
	_ = copy(workerWorld, world)

	turn := 0
	for turn < p.Turns {

		for i := 0; i < p.Threads; i++ {
			go worker(p, workerHeights[i], makeWorkerWorld(p, workerHeights[i], workerWorld, i), outChans[i])
		}

		newWorld := makeWorld(p.ImageHeight, p.ImageWidth)
		for i := range outChans {
			part := <-outChans[i]
			newWorld = append(newWorld, part...)
		}

		x := world
		world = newWorld
		newWorld = x

		turn++
	}

	aliveCells := calculateAliveCells(p, world)

	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}

	// Make sure that the IO has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
