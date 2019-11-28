package main

import "flag"

// golParams provides the details of how to run the Game of Life and which image to load.
type golParams struct {
	turns       int
	threads     int
	imageWidth  int
	imageHeight int
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type ioCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//		ioOutput 	= 0
//		ioInput 	= 1
//		ioCheckIdle = 2

const (
	ioOutput ioCommand = iota
	ioInput
	ioCheckIdle
)

// cell is used as the return type for the testing framework.
type cell struct {
	x, y int
}

// distributorToIo defines all chans that the distributor goroutine will have to communicate with the io goroutine.
// Note the restrictions on chans being send-only or receive-only to prevent bugs.
type distributorToIo struct {
	command   chan<- ioCommand
	idle      <-chan bool
	outputVal chan<- uint8
	filename  chan<- string
	inputVal  <-chan uint8
}

// ioToDistributor defines all chans that the io goroutine will have to communicate with the distributor goroutine.
// Note the restrictions on chans being send-only or receive-only to prevent bugs.
type ioToDistributor struct {
	command   <-chan ioCommand
	idle      chan<- bool
	outputVal <-chan uint8
	filename  <-chan string
	inputVal  chan<- uint8
}

// distributorChans stores all the chans that the distributor goroutine will use.
type distributorChans struct {
	io distributorToIo
}

// ioChans stores all the chans that the io goroutine will use.
type ioChans struct {
	distributor ioToDistributor
}

type outChans struct {
	tChan chan<- byte
	bChan chan<- byte
}

type inChans struct {
	tChan <-chan byte
	bChan <-chan byte
}

type chanType uint8

// It will evaluate to:
//		WORLD 	= 0
//		TOP 	= 1
//		BOTTOM  = 2
const (
	WORLD chanType = iota
	TOP
	BOTTOM
)

// gameOfLife is the function called by the testing framework.
// It makes some channels and starts relevant goroutines.
// It places the created channels in the relevant structs.
// It returns an array of alive cells returned by the distributor.
func gameOfLife(p golParams, key chan rune) []cell {
	var dChans distributorChans
	var ioChans ioChans

	ioCommand := make(chan ioCommand)
	dChans.io.command = ioCommand
	ioChans.distributor.command = ioCommand

	ioIdle := make(chan bool)
	dChans.io.idle = ioIdle
	ioChans.distributor.idle = ioIdle

	ioFilename := make(chan string)
	dChans.io.filename = ioFilename
	ioChans.distributor.filename = ioFilename

	inputVal := make(chan uint8)
	dChans.io.inputVal = inputVal
	ioChans.distributor.inputVal = inputVal

	outputVal := make(chan uint8)
	dChans.io.outputVal = outputVal
	ioChans.distributor.outputVal = outputVal

	aliveCells := make(chan []cell)
	workerChans := make([][]chan byte, p.threads)

	remainder := p.imageHeight % p.threads

	for i := 0; i < p.threads; i++ {
		workerChans[i] = make([]chan byte, 3)
		for j := 0; j < 3; j++ {
			workerChans[i][j] = make(chan byte)
		}

		var in inChans
		var out outChans

		in.tChan = workerChans[(i-1)%p.threads][TOP]
		in.bChan = workerChans[(i+1)%p.threads][BOTTOM]
		out.tChan = workerChans[i][TOP]
		out.bChan = workerChans[i][BOTTOM]

		if remainder > i {
			go worker(in, out, workerChans[i][WORLD], (p.imageHeight/p.threads + 3), p.imageWidth)

		} else {
			go worker(in, out, workerChans[i][WORLD], (p.imageHeight/p.threads + 2), p.imageWidth)
		}
	}

	go distributor(p, dChans, aliveCells, workerChans, key)
	go pgmIo(p, ioChans)

	alive := <-aliveCells
	return alive
}

// main is the function called when starting Game of Life with 'make gol'
// Do not edit until Stage 2.
func main() {
	var params golParams

	flag.IntVar(
		&params.threads,
		"t",
		8,
		"Specify the number of worker threads to use. Defaults to 8.")

	flag.IntVar(
		&params.imageWidth,
		"w",
		512,
		"Specify the width of the image. Defaults to 512.")

	flag.IntVar(
		&params.imageHeight,
		"h",
		512,
		"Specify the height of the image. Defaults to 512.")

	flag.Parse()

	params.turns = 1000000000

	key := make(chan rune)

	startControlServer(params)
	go getKeyboardCommand(key)
	gameOfLife(params, key)
	StopControlServer()
}
