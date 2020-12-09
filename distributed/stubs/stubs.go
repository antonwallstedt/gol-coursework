package stubs

var GameOfLifeHandler = "Engine.GameOfLife"

type Response struct {
	World [][]byte
	Turn  int
}

type Request struct {
	World [][]byte
	Turns int
}
