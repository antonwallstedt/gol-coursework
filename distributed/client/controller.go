package client

import "fmt"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	ioFilename := make(chan string)
	ioOutput := make(chan uint8)
	ioInput := make(chan uint8)

	distributorChannels := distributorChannels{
		events,
		ioCommand,
		ioIdle,
		ioFilename,
		ioOutput,
		ioInput,
	}
	go distributor(p, distributorChannels)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
	}
	go startIo(p, ioChannels)
}
func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func distributor(p Params, d distributorChannels) {
	d.ioCommand <- ioInput
	d.ioFilename <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)
	world := makeWorld(p.ImageHeight, p.ImageWidth)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-d.ioInput
		}
	}
}

// func makeCall(client rpc.Client, message string) {
// 	request := stubs.Request{Message: message}
// 	response := new(stubs.Response)
// 	client.Call(stubs.ReverseHandler, request, response)
// 	fmt.Println("Responded: " + response.Message)
// }

// func main() {
// 	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
// 	flag.Parse()
// 	client, _ := rpc.Dial("tcp", *server)
// 	defer client.Close()

// 	file, _ := os.Open("wordlist")
// 	for {

// 	}

// }
