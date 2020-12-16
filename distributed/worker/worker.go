package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

const (
	// ALIVE : pixel value for alive cells
	ALIVE = 255

	// DEAD : pixel value for dead cells
	DEAD = 0
)

func mod(x, m int) int {
	return (x + m) % m
}

func calculateNeighbours(imageHeight, imageWidth, x, y int, world [][]byte) int {
	neighbours := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				if world[mod(y+i, imageHeight)][mod(x+j, imageWidth)] == ALIVE {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func calculate(worldInput [][]byte, imageHeight, imageWidth, paramImageHeight int) [][]byte {
	fmt.Println(imageHeight)
	world := make([][]byte, imageHeight+2)
	for i := range world {
		world[i] = make([]byte, imageWidth)
	}
	if len(world) == len(worldInput) {
		world = worldInput
		fmt.Println("the world is equal in size.")
	} else {
		fmt.Println("the world is not equal")
	}

	newWorld := make([][]byte, imageHeight+2)
	for i := range world {
		newWorld[i] = make([]byte, imageWidth)
	}
	//we don't need to care about the first row, cause we need to ignore every first role.
	for y := 1; y <= imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			var neighboursAlive = 0
			neighboursAlive = calculateNeighbours(paramImageHeight, imageWidth, x, y, world)
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
	finishedWorld := make([][]byte, imageHeight)
	for i := range finishedWorld {
		finishedWorld[i] = make([]byte, imageWidth)
	}
	//Here is where we ignore the first and the last row.
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			finishedWorld[y][x] = newWorld[y+1][x]
		}
	}
	return finishedWorld
}

type Worker struct {
}

func (w *Worker) Calculate(req stubs.RequestWorkerWorld, res *stubs.ResponseWorkerWorld) (err error) {

	if req.World == nil {
		err = errors.New("a world must be specified")
		res.Message = "invalid world"
		return
	}

	newWorld := calculate(req.World, req.ImageHeight, req.ImageWidth, req.ParamsImageHeight)
	res.World = newWorld
	res.Message = "recieved workerWorld"
	return
}

var worker Worker

func main() {

	pAddr := flag.String("port", "8040", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&worker)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
