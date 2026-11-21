package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

// machineIDFromNodeID turns a Maelstrom node id like "n3" into 3.
func machineIDFromNodeID(nodeID string) (uint64, error) {
	id, err := strconv.ParseUint(strings.TrimPrefix(nodeID, "n"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected node id %q: %w", nodeID, err)
	}
	return id, nil
}

func main() {
	n := maelstrom.NewNode()

	// n.ID() is only known after Maelstrom sends "init", which happens inside
	// n.Run(), so the generator is created lazily on the first request.
	var (
		once    sync.Once
		gen     *snowflakeIdGenerator
		initErr error
	)
	getGenerator := func() (*snowflakeIdGenerator, error) {
		once.Do(func() {
			machineID, err := machineIDFromNodeID(n.ID())
			if err != nil {
				initErr = err
				return
			}
			gen, initErr = NewSnowflakeGenerator(machineID, generateTimeStamp())
		})
		return gen, initErr
	}

	n.Handle("generate", func(msg maelstrom.Message) error {
		g, err := getGenerator()
		if err != nil {
			return err
		}
		id, err := g.Generate()
		if err != nil {
			return err
		}
		// Sent as a string: n.Reply round-trips the body through
		// map[string]any, which decodes numbers as float64 and loses
		// precision above 2^53, turning distinct IDs into duplicates.
		return n.Reply(msg, map[string]any{
			"type": "generate_ok",
			"id":   strconv.FormatUint(id, 10),
		})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
