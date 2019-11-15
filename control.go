package main

import (
	"fmt"

	"github.com/nsf/termbox-go"
)

// getKeyboardCommand sends all keys pressed on the keyboard as runes (characters) on the key chan.
// getKeyboardCommand will NOT work if termbox isn't initialised (in startControlServer)
func getKeyboardCommand(key chan<- rune) {
	for {
		event := termbox.PollEvent()
		if event.Type == termbox.EventKey {
			if event.Key != 0 {
				key <- rune(event.Key)
			} else if event.Ch != 0 {
				key <- event.Ch
			}
		}
	}
}

// startControlServer initialises termbox and prints basic information about the game configuration.
func startControlServer(p golParams) {
	e := termbox.Init()
	check(e)

	fmt.Println("Threads:", p.threads)
	fmt.Println("Width:", p.imageWidth)
	fmt.Println("Height:", p.imageHeight)
}

// StopControlServer closes termbox.
// If the program is terminated without closing termbox the terminal window may misbehave.
func StopControlServer() {
	termbox.Close()
}

/* STAGE 2A

If s is pressed, generate a PGM file with the current state of the board.
If p is pressed, pause the processing and print the current turn that is being processed. If p is pressed again resume the processing and print "Continuing".
If q is pressed, generate a PGM file with the final state of the board and then terminate the program.

for {
	select {
	case Key := <- :
		switch rune {
		case s:

		case p:
			
		case q:

		}
	}
}
*/
