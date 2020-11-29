package gol

import (
	"fmt"
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
	//This will work when the world divide into worker world with the real integer and have mod 0

	for x := 0; x < imageWidth; x++ {
		workerWorld[0][x] = world[(currentThreads*workerHeight+imageHeight-1)%imageHeight][x]
	}
	//Check the last worker world to add remaining byte
	if currentThreads == Threads {
		modOfWorkerHeight := imageHeight / Threads
		// lastWokerHeight := modOfWorkerHeight + workerHeight
		var lastWokerHeight int
		//This is for 16*16 file, we need to add the mod from dividing total height and add that thing to last worker world
		if imageHeight == 16 {
			lastWokerHeight = modOfWorkerHeight + workerHeight

			for y := 1; y <= lastWokerHeight; y++ {
				for x := 0; x < imageWidth; x++ {
					workerWorld[y][x] = world[currentThreads*workerHeight+y-1][x]
				}
			}
			for x := 0; x < imageWidth; x++ {
				workerWorld[lastWokerHeight+1][x] = world[((currentThreads+1)*workerHeight+imageHeight)%imageHeight][x]
			}
			//I try to debug when the input file is 64*64, we need to add another byte to the last wokeWorld in order to get all the element in the world
		} else if imageHeight == 64 {
			lastWokerHeight = modOfWorkerHeight + 1 + workerHeight

			for y := 1; y <= lastWokerHeight; y++ {
				for x := 0; x < imageWidth; x++ {
					workerWorld[y][x] = world[currentThreads*workerHeight+y-1][x]
				}
			}
			for x := 0; x < imageWidth; x++ {
				workerWorld[lastWokerHeight+1][x] = world[((currentThreads+1)*workerHeight+imageHeight)%imageHeight][x]
			}
		}

	} else {

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
func aliveNeighbour(p Params, y, x int, world [][]byte) int {

	var a int
	var b int

	prevX := x - 1
	aftX := x + 1
	prevY := y - 1
	aftY := y + 1
	if x == 0 {
		prevX = p.ImageWidth - 1
	}
	if x == p.ImageWidth-1 {
		aftX = 0
	}
	if y == 0 {
		prevY = p.ImageHeight - 1
	}
	if y == p.ImageHeight-1 {
		aftY = 0
	}

	b = int(world[y][prevX]) + int(world[y][aftX]) + int(world[prevY][prevX]) + int(world[prevY][x]) + int(world[prevY][aftX]) + int(world[aftY][aftX]) + int(world[aftY][prevX]) + int(world[aftY][x])

	a = b / 255

	return a
}

//Worker is the functino that used to calculate the logic of the program and giving each byte of newWorld to distributor for finalComplete turn channel.
func worker(p Params, workerChan chan byte, imageHeight int, pImageHeight, imageWidth int, outChan chan byte, Thread, currentThread int) {
	// Create a world for worker to store the workerWorld, the reason why we plus two beacause we need to make sure that the we create the world to have same exact height as workerWorld height
	world := make([][]byte, imageHeight+2)
	for i := range world {
		world[i] = make([]byte, imageWidth)
	}
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			world[y][x] = <-workerChan
		}
	}

	newWorld := make([][]byte, imageHeight+2)
	for i := range world {
		newWorld[i] = make([]byte, imageWidth)
	}
	//we start from 1 because we dont wannt to check the first element
	for y := 1; y <= imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			var neighboursAlive = 0

			neighboursAlive = aliveNeighbour(p, y, x, world)
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
	// This one we dont want to send out the first element of the world.
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			outChan <- newWorld[y+1][x]
		}
	}

}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	var FinalTurnComplete FinalTurnComplete

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
		// Tiker for the 3rd steps
		ticker := time.NewTicker(2 * time.Second)
		select {
		// case <-ticker.C:
		// check the keyperess
		case k := <-keyPresses:
			if k == 's' {
				//s to start the board
				printBoard(c, p, world, turn)
			} else if k == 'q' {
				//q to quit the board
				printBoard(c, p, world, turn)
				fmt.Println("Terminated.")
				return
			} else if k == 'p' {
				//pausing the board
				fmt.Println(turn)
				fmt.Println("Pausing.")
				for {
					tempKey := <-keyPresses
					if tempKey == 'p' {
						fmt.Println("Continuing.")
						break
					}
				}
			}
		case <-ticker.C:
			var instanceAlive = 0
			for y := 0; y < p.ImageHeight; y++ {
				for x := 0; x < p.ImageWidth; x++ {
					if world[y][x] == 255 {
						instanceAlive += 1
					}
				}
			}

			fmt.Println("number of alive cells is:", instanceAlive)

		default:
		}
		var workerHeight int
		outChan := make([]chan byte, p.Threads)
		workerHeight = p.ImageHeight / p.Threads
		// modOfWorkerHeight := p.ImageWidth % p.Threads
		for i := 0; i < p.Threads; i++ {
			outChan[i] = make(chan byte)
			workerChan := make(chan byte)
			workerWorld := buildWorkerWorld(world, workerHeight, p.ImageHeight, p.ImageWidth, i, p.Threads)
			go worker(p, workerChan, workerHeight+2, p.ImageHeight, p.ImageWidth, outChan[i], p.Threads, i)
			//Send world cells to workers
			// if i == p.Threads {
			// 	for y := 0; y < workerHeight+modOfWorkerHeight+2; y++ {
			// 		for x := 0; x < p.ImageWidth; x++ {
			// 			workerChan <- workerWorld[y][x]
			// 		}
			// 	}
			// } else {
			for y := 0; y < workerHeight+2; y++ {
				for x := 0; x < p.ImageWidth; x++ {
					workerChan <- workerWorld[y][x]
				}
			}

		}
		//Receieving the thread to world
		for i := 0; i < p.Threads; i++ {
			//slices from workers
			// if i == p.Threads {
			// 	tempOut := make([][]byte, workerHeight+modOfWorkerHeight)
			// 	for i := range tempOut {
			// 		tempOut[i] = make([]byte, p.ImageWidth)
			// 	}
			// 	for y := 0; y < workerHeight+modOfWorkerHeight; y++ {
			// 		for x := 0; x < p.ImageWidth; x++ {
			// 			world[i*workerHeight+modOfWorkerHeight+y][x] = tempOut[y][x]
			// 		}
			// 	}

			// } else {
			tempOut := make([][]byte, workerHeight)
			for i := range tempOut {
				tempOut[i] = make([]byte, p.ImageWidth)
			}
			//Recieving the byte to the temp world for the first thread
			for y := 0; y < workerHeight; y++ {
				for x := 0; x < p.ImageWidth; x++ {
					tempOut[y][x] = <-outChan[i]
				}
			}
			//String the temp world in to world
			for y := 0; y < workerHeight; y++ {
				for x := 0; x < p.ImageWidth; x++ {
					world[i*workerHeight+y][x] = tempOut[y][x]
				}
			}
			// }
		}
		turn++
	}
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == ALIVE {
				listCell = append(listCell, util.Cell{X: x, Y: y})
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
	close(c.events)
}
func printBoard(d distributorChannels, p Params, world [][]byte, turn int) {

	d.ioCommand <- ioOutput
	d.ioFileName <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)
}
