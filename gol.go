package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

//WORKER FUNCTIONS

func worker(in inChans, out outChans, wChan chan byte, height int, width int) {
	//Create empty slice for world chunk WITH
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}

	for {
		for i := 1; i < height-1; i++ {
			for j := 0; j < width; j++ {
				world[i][j] = <-wChan
			}
		}

		//if deadlock try 2 for loops
		for j := 0; j < width; j++ {
			out.tChan <- world[1][j]
			out.bChan <- world[height-1][j]
			world[0][j] = <-in.tChan
			world[height-1][j] = <-in.bChan
		}

		world = makeTurn(world, height, width)
	}
}

func makeTurn(world [][]byte, height int, width int) [][]byte {
	//Create new empty world slice
	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}

	//Fill new empty world with alive cells
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			//For each cell count surrounding alive cells
			count := 0
			for i := -1; i < 2; i++ {
				for j := -1; j < 2; j++ {
					if !(i == 0 && j == 0) {
						if world[(y+height+i)%height][(x+width+j)%width] != 0 {
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

	//Return newWorld
	return newWorld
}

//DISTRIBUTOR FUNCTIONS

//findAlive returns a list of alive cells
func findAlive(p golParams, d distributorChans, world [][]byte) []cell {
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

//Send slice of original image to worker, and waits to receive new image
func sendSliceToWorkerAndReceive(p golParams, workerChans [][]chan byte, world [][]byte) {
	// Sends slice to worker

	bounds := make([][]int, p.threads)
	for i := 0; i < p.threads; i++ {
		bounds[i] = make([]int, 2)
	}

	offset := 0
	for thread := 0; thread < p.threads; thread++ {
		top := (thread*(p.imageHeight/p.threads) - 1)
		bottom := ((thread + 1) * (p.imageHeight / p.threads))
		if (p.imageHeight % p.threads) > thread {
			top += offset
			bottom += offset + 1
			offset++
		} else {
			top += offset
			bottom += offset
		}

		bounds[thread][0] = top
		bounds[thread][1] = bottom
		for i := top; i <= bottom; i++ {
			for j := 0; j <= p.imageWidth-1; j++ {
				workerChans[thread][WORLD] <- world[(i+p.imageHeight)%p.imageHeight][j]
			}
		}
	}

	//receive slice from worker
	for thread := 0; thread < p.threads; thread++ {
		for i := bounds[thread][0] + 1; i < bounds[thread][1]; i++ {
			//fmt.Println(i)
			for j := 0; j < p.imageWidth; j++ {
				world[i][j] = <-workerChans[thread][WORLD]
			}
		}
	}
}

func outputPgmImage(p golParams, d distributorChans, world [][]byte, turn int) {
	//Request pgmIo goroutine to output 2D slice as image
	fmt.Println("Output in progress...")
	d.io.command <- ioOutput
	d.io.filename <- strconv.Itoa(p.imageHeight) + "x" + strconv.Itoa(p.imageWidth) + "-" + strconv.Itoa(turn)

	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[x][y] //Sends to channel for io to receive
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell, workerChans [][]chan byte, key chan rune) {

	// Create the 2D slice to store the world.
	world := make([][]byte, p.imageHeight)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")

	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	// Program state enum
	type progState uint8

	const (
		STOP progState = iota
		PAUSE
		CONTINUE
	)
	state := CONTINUE

	timer := time.NewTicker(2 * time.Second)

	for turn := 0; (turn < p.turns) && (state == CONTINUE); {

		select {
		case <-timer.C:
			alive := findAlive(p, d, world)
			fmt.Println("Alive cells: ", len(alive))
		case runeInt := <-key:
			rune := string(runeInt)
			switch rune {
			case "s":
				outputPgmImage(p, d, world, turn)
			case "p":
				fmt.Println("Waiting...")
				state = PAUSE
				for state == PAUSE {
					runeInt = <-key
					rune = string(runeInt)

					switch rune {
					case "s":
						outputPgmImage(p, d, world, turn)
					case "q":
						state = STOP
					case "p":
						fmt.Println("Continuing...")
						state = CONTINUE
					}
				}
			case "q":
				state = STOP
			}

		default:
			// Sends slice to worker and receive after logic
			sendSliceToWorkerAndReceive(p, workerChans, world)
			turn++
		}
		//fmt.Println(turn)
	}

	//Request pgmIo goroutine to output 2D slice as image
	outputPgmImage(p, d, world, p.turns)

	// Go through the world and append the cells that are still alive.
	var finalAlive []cell = findAlive(p, d, world)

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
