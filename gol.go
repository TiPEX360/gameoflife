package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func worker(in inChans, out outChans, wChan chan byte, height int, width int, coms chan workerComs) {
	// World slice for the worker INCLUDING HALOS
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}

	for {
		select {
		case command := <-coms: //Assign new command if available
			switch command {
			case INPUT:
				for y := 1; y < height-1; y++ {
					for x := 0; x < width; x++ {
						world[y][x] = <-wChan
					}
				}
			case OUTPUT:
				for y := 1; y < height-1; y++ {
					for x := 0; x < width; x++ {
						wChan <- world[y][x]
					}
				}
			case WORK:
				for x := 0; x < width; x++ {
					out.tChan <- world[1][x]
					out.bChan <- world[height-2][x]
					world[0][x] = <-in.tChan
					world[height-1][x] = <-in.bChan
				}
				world = makeTurn(world, height, width)
			}
		}
	}
}

// Processes game logic on a given slice
func makeTurn(world [][]byte, height int, width int) [][]byte {
	//Create new empty world slice
	newWorld := make([][]byte, height)
	for y := range newWorld {
		newWorld[y] = make([]byte, width)
		for x := 0; x < width; x++ {
			//For each cell count surrounding alive cells
			count := 0
			for i := -1; i < 2; i++ {
				for j := -1; j < 2; j++ {
					if !(i == 0 && j == 0) {

						//  if world[(y+height+i)%height][(x+width+j)%width] != 0 {
						yFinal := y + i + height
						xFinal := x + j + width
						for yFinal >= height {
							yFinal -= height
						}
						for xFinal >= width {
							xFinal -= width
						}
						if world[yFinal][xFinal] != 0 {
							count++
						}
					}
				}
			}

			//Update current cells in new World
			switch {
			case count < 2:
				newWorld[y][x] = 0x00
			case count == 3:
				newWorld[y][x] = 0xFF
			case count == 2:
				newWorld[y][x] = world[y][x]
			}
		}
	}

	// //Fill new empty world with alive cells
	// for y := 0; y < height; y++ {
	// }

	return newWorld
}

// Returns the inclusive bounds that each worker should process. Does not include halos. || Could this be optimized?
func findBounds(p golParams) [][]int {
	bounds := make([][]int, p.threads)
	for i := 0; i < p.threads; i++ {
		bounds[i] = make([]int, 2)
	}

	offset := 0
	//OPTIMIZE
	//if (p.imageHeight % p.threads) > thread {
	remainder := p.imageHeight
	for remainder >= p.threads {
		remainder -= p.threads
	}
	for thread := 0; thread < p.threads; thread++ {
		top := (thread * (p.imageHeight / p.threads)) + offset
		bottom := ((thread+1)*(p.imageHeight/p.threads) - 1) + offset

		//
		if remainder > thread {
			bottom++
			offset++
		}

		bounds[thread][0] = top
		bounds[thread][1] = bottom
	}
	return bounds
}

// Returns an array of alive cells in a given world
func findAlive(p golParams, world [][]byte) []cell {
	var alive []cell
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				alive = append(alive, cell{x: x, y: y}) //Sets finalAlive for testing
			}
		}
	}
	return alive
}

// Sends the current world section to each worker
func sendWorld(p golParams, workerChans [][]chan byte, world [][]byte, bounds [][]int) {
	for thread := 0; thread < p.threads; thread++ {
		top := bounds[thread][0]
		bottom := bounds[thread][1]
		for y := top; y <= bottom; y++ {
			for x := 0; x < p.imageWidth; x++ {
				workerChans[thread][WORLD] <- world[y][x]
			}
		}
	}
}

// Receives the world from all workers
func receiveWorld(p golParams, workerChans [][]chan byte, world [][]byte, bounds [][]int) {
	for thread := 0; thread < p.threads; thread++ {
		for y := bounds[thread][0]; y <= bounds[thread][1]; y++ {
			for x := 0; x < p.imageWidth; x++ {
				world[y][x] = <-workerChans[thread][WORLD]
			}
		}
	}
}

// Outputs current world as PGM
func outputPgmImage(p golParams, d distributorChans, world [][]byte) {
	//Request pgmIo goroutine to output 2D slice as image
	fmt.Println("Output in progress...")
	d.io.command <- ioOutput
	d.io.filename <- strconv.Itoa(p.imageHeight) + "x" + strconv.Itoa(p.imageWidth) + "-" + strconv.Itoa(p.turns)

	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[x][y] //Sends to channel for io to receive
		}
	}
}

// Sends a given command to each worker
func sendCommand(p golParams, comChans []chan workerComs, command workerComs) {
	for i := 0; i < p.threads; i++ {
		comChans[i] <- command
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell, workerChans [][]chan byte, key chan rune, comChans []chan workerComs) {

	// Create the 2D slice to store the world.
	world := make([][]byte, p.imageHeight)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	bounds := findBounds(p)

	//Send initial world to workers
	sendCommand(p, comChans, INPUT)
	sendWorld(p, workerChans, world, bounds)

	timer := time.NewTicker(2 * time.Second)

	state := CONTINUE
	for turn := 0; (turn < p.turns) && (state == CONTINUE); {
		select {
		case <-timer.C:
			sendCommand(p, comChans, OUTPUT)
			receiveWorld(p, workerChans, world, bounds)
			alive := findAlive(p, world)
			fmt.Println("Alive cells: ", len(alive))
		case runeInt := <-key:
			rune := string(runeInt)
			switch rune {
			case "s":
				sendCommand(p, comChans, OUTPUT)
				receiveWorld(p, workerChans, world, bounds)
				outputPgmImage(p, d, world)
			case "p":
				state = PAUSE
				fmt.Println("Waiting...")
				for state == PAUSE {
					runeInt = <-key
					rune = string(runeInt)
					switch rune {
					case "s":
						sendCommand(p, comChans, OUTPUT)
						receiveWorld(p, workerChans, world, bounds)
						outputPgmImage(p, d, world)
					case "q":
						state = STOP
					case "p":
						state = CONTINUE
						fmt.Println("Continuing...")
					}
				}
			case "q":
				state = STOP
			}
		default:
			sendCommand(p, comChans, WORK)
			turn++
		}
	}

	// Receive world after all turns have been completed
	sendCommand(p, comChans, OUTPUT)
	receiveWorld(p, workerChans, world, bounds)
	outputPgmImage(p, d, world)

	// Go through the world and append the cells that are still alive.
	finalAlive := findAlive(p, world)

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
