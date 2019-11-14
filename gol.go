package main

import (
	"fmt"
	"strconv"
	"strings"
)

func worker(wChan chan byte, height int, width int) {
	//Create empty slice for world chunk
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}

	for {
		for i := 0; i < height; i++ {
			for j := 0; j < width; j++ {
				world[i][j] = <-wChan
			}
		}

		world = makeTurn(world, height, width)

		for i := 1; i < height-1; i++ {
			for j := 0; j < width; j++ {
				wChan <- world[i][j]
			}
		}
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

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell, workerChans []chan byte) {
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

	// Calculate the new state of Game of Life after the given number of turns.
	for turns := 0; turns < p.turns; turns++ {
		for thread := 0; thread < p.threads; thread++ {
			top := (thread*(p.imageHeight/p.threads) - 1)
			bottom := ((thread + 1) * (p.imageHeight / p.threads))
			for i := top; i <= bottom; i++ {
				for j := 0; j <= p.imageWidth-1; j++ {

					workerChans[thread] <- world[(i+p.imageHeight)%p.imageHeight][j]
				}
			}
		}

		for thread := 0; thread < p.threads; thread++ {
			for i := thread * (p.imageHeight / p.threads); i < (thread+1)*(p.imageHeight/p.threads); i++ {
				for j := 0; j < p.imageWidth; j++ {
					world[i][j] = <-workerChans[thread]
				}
			}
		}
	}

	//Request pgmIo goroutine to output 2D slice as image
	d.io.command <- ioOutput
	d.io.filename <- strconv.Itoa(p.imageHeight) + "x" + strconv.Itoa(p.imageWidth) + "-" + strconv.Itoa(p.turns)
	// Create an empty slice to store coordinates of cells that a1re still alive after p.turns are done.
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[x][y] //Sends to channel for io to receive
			if world[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y}) //Sets finalAlive for testing
			}
		}
	}

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive

}
