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

/* printWorld : TEMPORARY DEBUG FUNCTION, REMOVE ME */
func printWorld(world [][]byte) {
	for _, row := range world {
		fmt.Println(row)
	}
}

func worker(id int, p Params, workerHeight int, topRowChan, bottomRowChan chan []byte, in <-chan []byte, out chan<- [][]byte, parity int) {

	worldPart := makeWorld(workerHeight+2, p.ImageWidth)
	newWorldPart := makeWorld(workerHeight, p.ImageWidth)
	for y := 1; y <= workerHeight; y++ {
		worldPart[y] = <-in
	}

	for turn := 0; turn < p.Turns; turn++ {
		if parity == 1 {
			bottomRowChan <- worldPart[workerHeight]
			worldPart[0] = <-topRowChan
			topRowChan <- worldPart[1]
			worldPart[workerHeight+1] = <-bottomRowChan
		} else {
			worldPart[0] = <-topRowChan
			bottomRowChan <- worldPart[workerHeight]
			worldPart[workerHeight+1] = <-bottomRowChan
			topRowChan <- worldPart[1]
		}
		worldPart = calculateNextState(p, worldPart)
	}

	for y := 1; y <= workerHeight; y++ {
		newWorldPart[y-1] = worldPart[y]
	}

	out <- newWorldPart

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

	// Calculate heights
	defaultHeight := p.ImageHeight / p.Threads
	workerHeights := make([]int, p.Threads)
	for i := range workerHeights {
		workerHeights[i] = defaultHeight
	}

	remainder := p.ImageHeight % p.Threads
	if remainder != 0 {
		workerHeights[len(workerHeights)-1] += remainder
	}

	// Create all channels
	outChans := make([]chan [][]byte, p.Threads)
	inChans := make([]chan []byte, p.Threads)
	haloChans := make([]chan []byte, p.Threads)

	for i := 0; i < p.Threads; i++ {
		outChans[i] = make(chan [][]byte)
		inChans[i] = make(chan []byte)
		haloChans[i] = make(chan []byte, p.ImageHeight)
	}

	// Start workers
	for i := 0; i < p.Threads; i++ {
		go worker(i, p, workerHeights[i], haloChans[(i-1+p.Threads)%p.Threads], haloChans[i], inChans[i], outChans[i], i%2)
	}

	// Send data to workers
	for i := range inChans {
		if i != p.Threads-1 {
			for y := i * workerHeights[i]; y < (i+1)*workerHeights[i]; y++ {
				inChans[i] <- world[y]
			}
		} else {
			for y := i * defaultHeight; y < p.ImageHeight; y++ {
				inChans[i] <- world[y]
			}
		}
	}

	// Receive data from workers
	newWorld := makeWorld(0, 0)
	for i := range outChans {
		worldPart := <-outChans[i]
		newWorld = append(newWorld, worldPart...)
	}

	// Update world
	x := world
	world = newWorld
	newWorld = x

	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.

	// TODO: remove this and come up with another way of getting the value of turns from each worker back, as the turns loop
	// 		 is now running inside each individual worker
	turn := 0
	for turn < p.Turns {
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
