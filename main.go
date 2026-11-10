// Command localrdp opens a Windows child session as a second desktop.
package main

import (
	"os"

	"localrdp/internal/rdp"
)

func main() {
	os.Exit(rdp.Run(os.Args))
}
