package snowflake

import "github.com/bwmarrin/snowflake"

var node *snowflake.Node

func Init(nodeID int64) error {
	var err error
	node, err = snowflake.NewNode(nodeID)
	return err
}

func GenerateID() int64 {
	if node == nil {
		// Fallback: initialize with node 1
		_ = Init(1)
	}
	return node.Generate().Int64()
}
