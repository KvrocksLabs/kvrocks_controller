package storage

import (
	"fmt"
	"errors"

	"github.com/KvrocksLabs/kvrocks-controller/metadata"
)

// ListNodes return the list of nodes under the specified shard
func (stor *Storage) ListNodes(ns, cluster string, shardIdx int) ([]metadata.NodeInfo, error) {
	stor.rw.RLock()
	defer stor.rw.RUnlock()
	if !stor.selfLeaderWithUnLock() {
		return nil, ErrSlaveNoSupport
	}
	shard, err := stor.getShard(ns, cluster, shardIdx)
	if err != nil {
		return nil, fmt.Errorf("get shard: %w", err)
	}
	nodes := make([]metadata.NodeInfo, 0, len(shard.Nodes))
	copy(nodes, shard.Nodes)
	return nodes, nil
}

// CreateNode add a node under the specified shard
func (stor *Storage) CreateNode(ns, cluster string, shardIdx int, node *metadata.NodeInfo) error {
	stor.rw.Lock()
	defer stor.rw.Unlock()
	if !stor.selfLeaderWithUnLock() {
		return ErrSlaveNoSupport
	}
	topo, err := stor.local.GetClusterCopy(ns, cluster)
	if err != nil {
		return fmt.Errorf("get cluster: %w", err)
	}
	if shardIdx >= len(topo.Shards) || shardIdx < 0 {
		return metadata.ErrShardIndexOutOfRange
	}
	shard := topo.Shards[shardIdx]
	if shard.Nodes == nil {
		shard.Nodes = make([]metadata.NodeInfo, 0)
	}
	for _, existedNode := range shard.Nodes {
		if existedNode.Address == node.Address {
			return metadata.NewError("existedNode", metadata.CodeExisted, "")
		}
	}
	// NodeRole check
	if len(shard.Nodes) == 0 && !node.IsMaster() {
		return errors.New("you MUST add master node first")
	}
	if len(shard.Nodes) != 0 && node.IsMaster() {
		return errors.New("the master node has already added in this shard")
	}
	topo.Version++
	topo.Shards[shardIdx].Nodes = append(topo.Shards[shardIdx].Nodes, *node)
	if err := stor.updateCluster(ns, cluster, &topo); err != nil {
		return err
	}
	stor.EmitEvent(Event{
		Namespace: ns,
		Cluster:   cluster,
		Shard:     shardIdx,
		NodeID:    node.ID,
		Type:      EventNode,
		Command:   CommandCreate,
	})
	return nil
}

// RemoveNode delete the node from the specified shard
func (stor *Storage) RemoveNode(ns, cluster string, shardIdx int, nodeID string) error {
	stor.rw.Lock()
	defer stor.rw.Unlock()
	if !stor.selfLeaderWithUnLock() {
		return ErrSlaveNoSupport
	}
	topo, err := stor.local.GetClusterCopy(ns, cluster)
	if err != nil {
		return fmt.Errorf("get cluster: %w", err)
	}
	if shardIdx >= len(topo.Shards) || shardIdx < 0 {
		return metadata.ErrShardIndexOutOfRange
	}
	shard := topo.Shards[shardIdx]
	if shard.Nodes == nil {
		return metadata.NewError("node", metadata.CodeNoExists, "")
	}
	nodeIdx := -1
	for idx, node := range shard.Nodes {
		if node.ID == nodeID {
			nodeIdx = idx
			break
		}
	}
	if nodeIdx == -1 {
		return metadata.NewError("node", metadata.CodeNoExists, "")
	}
	node := shard.Nodes[nodeIdx]
	if len(shard.SlotRanges) != 0 {
		if len(shard.Nodes) == 1 || node.IsMaster() {
			return errors.New("still some slots in this shard, please migrate them first")
		}
	} else {
		if node.IsMaster() && len(shard.Nodes) > 1 {
			return errors.New("please remove slave Shards first")
		}
	}
	topo.Version++
	topo.Shards[shardIdx].Nodes = append(topo.Shards[shardIdx].Nodes[:nodeIdx], topo.Shards[shardIdx].Nodes[nodeIdx+1:]...)
	if err := stor.updateCluster(ns, cluster, &topo); err != nil {
		return err
	}
	stor.EmitEvent(Event{
		Namespace: ns,
		Cluster:   cluster,
		Shard:     shardIdx,
		NodeID:    node.ID,
		Type:      EventNode,
		Command:   CommandRemove,
	})
	return nil
}

// UpdateNode update the exist node under the specified shard
func (stor *Storage) UpdateNode(ns, cluster string, shardIdx int, node *metadata.NodeInfo) error {
	stor.rw.Lock()
	defer stor.rw.Unlock()
	if !stor.selfLeaderWithUnLock() {
		return ErrSlaveNoSupport
	}
	topo, err := stor.local.GetClusterCopy(ns, cluster)
	if err != nil {
		return fmt.Errorf("get cluster: %w", err)
	}
	if shardIdx >= len(topo.Shards) || shardIdx < 0 {
		return metadata.ErrShardIndexOutOfRange
	}
	shard := topo.Shards[shardIdx]
	if shard.Nodes == nil {
		return metadata.ErrNodeNoExists
	}
	// TODO: check the role
	for idx, existedNode := range shard.Nodes {
		if existedNode.ID == node.ID {
			topo.Version++
			topo.Shards[shardIdx].Nodes[idx] = *node
			if err := stor.updateCluster(ns, cluster, &topo); err != nil {
				return err
			}
			stor.EmitEvent(Event{
				Namespace: ns,
				Cluster:   cluster,
				Shard:     shardIdx,
				NodeID:    node.ID,
				Type:      EventNode,
				Command:   CommandUpdate,
			})
			return nil
		}
	}
	return metadata.NewError("node", metadata.CodeNoExists, "")
}
