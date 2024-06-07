package client

import (
	"log"
	"os"
)

var debuglog = log.New(os.Stderr, "client.pproxit: ", log.Ldate)