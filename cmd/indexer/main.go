package main

import (
	"log"
	"time"
)

// Placeholder indexer: in real implementation, subscribe to chaincode events
// via Fabric Gateway and upsert to Postgres.
func main() {
	log.Println("Indexer starting (placeholder). TODO: connect to Fabric and Postgres.")
	for {
		log.Println("tick: waiting for events...")
		time.Sleep(10 * time.Second)
	}
}
