package worker

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

const ALIVE = 255
const DEAD = 0

var workChan = make(chan stubs.Work)

func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func mod(x, m int) int {
	return (x + m) % m
}

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

func calculateNextState(p gol.Params, world [][]byte) [][]byte {
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

func calculateAliveCells(p gol.Params, world [][]byte) []util.Cell {
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

func getOutboundIP() string {
	conn, _ := net.Dial("udp", "8.8.8.8:80")
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr).IP.String()
	return localAddr
}

func makeStep(ch chan [][]byte, client *rpc.Client) {
	for {
		work := <-ch
		newWorld := stubs.Work{World: work.World, Parameters: work.Parameters}
		toWork := stubs.PublishRequest{Topic: "step", Work: newWorld}
		status := new(stubs.StatusReport)
		err := client.Call(stubs.Publish, toWork, status)
		if err != nil {
			fmt.Println("RPC client returned error:")
			fmt.Println(err)
			fmt.Println("Dropping step.")
		}
	}
}

type Worker struct{}

func (w *Worker) Step(req stubs.Work, res *stubs.JobReport) (err error) {
	res.Result = calculateNextState(req.Parameters, req.World)
	fmt.Println("Calculated next step")
	workChan <- res.Result
	return
}

func main() {
	pAddr := flag.String("port", "8050", "Port to listen on")
	engineAddr := flag.String("engine", "127.0.0.1:8030", "Address of engine instance")
	flag.Parse()
	client, _ := rpc.Dial("tcp", *engineAddr)
	status := new(stubs.StatusReport)
	client.Call(stubs.CreateChannel, stubs.ChannelRequest{Topic: "step", Buffer: 10}, status)
	rpc.Register(&Worker{})
	fmt.Println(*pAddr)
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println(err)
	}
	client.Call(stubs.Subscribe, stubs.Subscription{Topic: "step", WorkerAddress: getOutboundIP() + ":" + *pAddr, Callback: "Worker.Step"}, status)
	defer listener.Close()
	go makeStep(workChan)
}

/*
func main(){
	pAddr := flag.String("port","8050","Port to listen on")
	brokerAddr := flag.String("broker","127.0.0.1:8030", "Address of broker instance")
	flag.Parse()
	client, _ := rpc.Dial("tcp", *brokerAddr)
	status := new(stubs.StatusReport)
	client.Call(stubs.CreateChannel, stubs.ChannelRequest{Topic: "multiply", Buffer: 10}, status)
	client.Call(stubs.CreateChannel, stubs.ChannelRequest{Topic: "divide", Buffer: 10}, status)
	rpc.Register(&Factory{})
	fmt.Println(*pAddr)
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println(err)
	}
	client.Call(stubs.Subscribe, stubs.Subscription{Topic: "multiply", FactoryAddress: getOutboundIP()+":"+*pAddr, Callback: "Factory.Multiply"}, status)
	client.Call(stubs.Subscribe, stubs.Subscription{Topic: "divide", FactoryAddress: getOutboundIP()+":"+*pAddr, Callback: "Factory.Divide"}, status)
	defer listener.Close()
	go makedivision(mulch, client)
	rpc.Accept(listener)
	flag.Parse()
}

*/
