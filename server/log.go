package server

import (
	"log"
	"os"
)

var debuglog = log.New(os.Stderr, "server.pproxit: ", log.Ldate)
