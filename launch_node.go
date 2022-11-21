package main

import (
	"vocdoni-challenge/node"
	flag "github.com/spf13/pflag"
)

func main() {

	port := flag.Int16("port", 6643, "port to up node")
	groupKey := flag.String("groupKey", "test", "topic to connect node to network")
	flag.Parse()

	node.StartNode(*groupKey, *port)

}