package stubs

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

var NextStateHandler = "CalculateNextStateOperation.distributor"

type Response struct {
	NewWorld [][]byte
}

type Request struct {
	World [][]byte
	Turns int
}
