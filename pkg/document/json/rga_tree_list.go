/*
 * Copyright 2020 The Yorkie Authors. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package json

import (
	"strings"

	"github.com/yorkie-team/yorkie/pkg/document/time"
	"github.com/yorkie-team/yorkie/pkg/splay"
)

// RGATreeListNode is a node of RGATreeList.
type RGATreeListNode struct {
	indexNode *splay.Node
	elem      Element

	prev *RGATreeListNode
	next *RGATreeListNode
}

func newRGATreeListNode(elem Element) *RGATreeListNode {
	node := &RGATreeListNode{
		prev: nil,
		next: nil,
		elem: elem,
	}
	node.indexNode = splay.NewNode(node)

	return node
}

func newRGATreeListNodeAfter(prev *RGATreeListNode, elem Element) *RGATreeListNode {
	newNode := newRGATreeListNode(elem)
	prevNext := prev.next

	prev.next = newNode
	newNode.prev = prev
	newNode.next = prevNext
	if prevNext != nil {
		prevNext.prev = newNode
	}

	return prev.next
}

// Element returns the element of this node.
func (n *RGATreeListNode) Element() Element {
	return n.elem
}

// CreatedAt returns the creation time of this element.
func (n *RGATreeListNode) CreatedAt() time.Ticket {
	return n.elem.CreatedAt()
}

// PositionedAt returns the time this element was positioned.
func (n *RGATreeListNode) PositionedAt() time.Ticket {
	if n.elem.MovedAt() != nil {
		return *n.elem.MovedAt()
	}

	return n.elem.CreatedAt()
}

// Len returns the length of this node.
func (n *RGATreeListNode) Len() int {
	if n.isRemoved() {
		return 0
	}
	return 1
}

// String returns the string representation of this node.
func (n *RGATreeListNode) String() string {
	return n.elem.Marshal()
}

func (n *RGATreeListNode) isRemoved() bool {
	return n.elem.RemovedAt() != nil
}

// RGATreeList is a list with improved index-based lookup in RGA. RGA is a
// linked list that has a logical clock and tombstone. Since RGA is composed as
// a linked list, index-based element search is slow, O(n). To optimise for fast
// insertions and removals at any index in the list, RGATreeList has a tree.
type RGATreeList struct {
	dummyHead          *RGATreeListNode
	last               *RGATreeListNode
	size               int
	nodeMapByIndex     *splay.Tree
	nodeMapByCreatedAt map[string]*RGATreeListNode
}

// NewRGATreeList creates a new instance of RGATreeList.
func NewRGATreeList() *RGATreeList {
	dummyValue := NewPrimitive(0, time.InitialTicket)
	dummyValue.SetRemovedAt(&time.InitialTicket)
	dummyHead := newRGATreeListNode(dummyValue)
	nodeMapByIndex := splay.NewTree(dummyHead.indexNode)
	nodeMapByCreatedAt := make(map[string]*RGATreeListNode)
	nodeMapByCreatedAt[dummyHead.CreatedAt().Key()] = dummyHead

	return &RGATreeList{
		dummyHead:          dummyHead,
		last:               dummyHead,
		size:               0,
		nodeMapByIndex:     nodeMapByIndex,
		nodeMapByCreatedAt: nodeMapByCreatedAt,
	}
}

// Marshal returns the JSON encoding of this RGATreeList.
func (a *RGATreeList) Marshal() string {
	sb := strings.Builder{}
	sb.WriteString("[")

	current := a.dummyHead.next
	for {
		if current == nil {
			break
		}

		if !current.isRemoved() {
			sb.WriteString(current.elem.Marshal())

			// FIXME: When the last element of the array is deleted, it does not
			// work properly.
			if current != a.last {
				sb.WriteString(",")
			}
		}

		current = current.next
	}

	sb.WriteString("]")

	return sb.String()
}

// Add adds the given element at the last.
func (a *RGATreeList) Add(elem Element) {
	a.insertAfter(a.last.CreatedAt(), elem, elem.CreatedAt())
}

// Nodes returns an array of elements contained in this RGATreeList.
// TODO: If we encounter performance issues, we need to replace this with other solution.
func (a *RGATreeList) Nodes() []*RGATreeListNode {
	var nodes []*RGATreeListNode
	current := a.dummyHead.next
	for {
		if current == nil {
			break
		}
		nodes = append(nodes, current)
		current = current.next
	}

	return nodes
}

// LastCreatedAt returns the creation time of last elements.
func (a *RGATreeList) LastCreatedAt() time.Ticket {
	return a.last.CreatedAt()
}

// InsertAfter inserts the given element after the given previous element.
func (a *RGATreeList) InsertAfter(prevCreatedAt time.Ticket, elem Element) {
	a.insertAfter(prevCreatedAt, elem, elem.CreatedAt())
}

// Get returns the element of the given index.
func (a *RGATreeList) Get(idx int) *RGATreeListNode {
	splayNode, offset := a.nodeMapByIndex.Find(idx)
	node := splayNode.Value().(*RGATreeListNode)

	if idx == 0 && splayNode == a.dummyHead.indexNode {
		for {
			node = node.next
			if !node.isRemoved() {
				break
			}
		}
	} else if offset > 0 {
		for {
			node = node.next
			if !node.isRemoved() {
				break
			}
		}
	}

	return node
}

// DeleteByCreatedAt deletes the given element.
func (a *RGATreeList) DeleteByCreatedAt(createdAt time.Ticket, deletedAt *time.Ticket) *RGATreeListNode {
	node, ok := a.nodeMapByCreatedAt[createdAt.Key()]
	if !ok {
		panic("fail to find the given createdAt: " + createdAt.Key())
	}

	alreadyRemoved := node.isRemoved()
	if node.elem.Remove(deletedAt) && !alreadyRemoved {
		a.nodeMapByIndex.Splay(node.indexNode)
		a.size--
	}
	return node
}

// Len returns length of this RGATreeList.
func (a *RGATreeList) Len() int {
	return a.size
}

// AnnotatedString returns a String containing the metadata of the node id
// for debugging purpose.
func (a *RGATreeList) AnnotatedString() string {
	return a.nodeMapByIndex.AnnotatedString()
}

// Delete deletes the node of the given index.
func (a *RGATreeList) Delete(idx int, deletedAt *time.Ticket) *RGATreeListNode {
	target := a.Get(idx)
	return a.DeleteByCreatedAt(target.CreatedAt(), deletedAt)
}

// MoveAfter moves the given `createdAt` element after the `prevCreatedAt`
// element.
func (a *RGATreeList) MoveAfter(prevCreatedAt, createdAt, executedAt time.Ticket) {
	prevNode, ok := a.nodeMapByCreatedAt[prevCreatedAt.Key()]
	if !ok {
		panic("fail to find the given prevCreatedAt: " + prevCreatedAt.Key())
	}

	node, ok := a.nodeMapByCreatedAt[createdAt.Key()]
	if !ok {
		panic("fail to find the given createdAt: " + createdAt.Key())
	}

	if node.elem.MovedAt() == nil || executedAt.After(*node.elem.MovedAt()) {
		a.release(node)
		a.insertAfter(prevNode.CreatedAt(), node.elem, executedAt)
		node.elem.SetMovedAt(&executedAt)
	}
}

// FindPrevCreatedAt returns the creation time of the previous element of the
// given element.
func (a *RGATreeList) FindPrevCreatedAt(createdAt time.Ticket) time.Ticket {
	node, ok := a.nodeMapByCreatedAt[createdAt.Key()]
	if !ok {
		panic("fail to find the given prevCreatedAt: " + createdAt.Key())
	}

	for {
		node = node.prev
		if a.dummyHead == node || !node.isRemoved() {
			break
		}
	}

	return node.CreatedAt()
}

// purge physically purge child element.
func (a *RGATreeList) purge(elem Element) {
	node, ok := a.nodeMapByCreatedAt[elem.CreatedAt().Key()]
	if !ok {
		panic("fail to find the given createdAt: " + elem.CreatedAt().Key())
	}

	a.release(node)
}

func (a *RGATreeList) findNextBeforeExecutedAt(
	createdAt time.Ticket,
	executedAt time.Ticket,
) *RGATreeListNode {
	node, ok := a.nodeMapByCreatedAt[createdAt.Key()]
	if !ok {
		panic("fail to find the given createdAt: " + createdAt.Key())
	}

	for node.next != nil && node.next.PositionedAt().After(executedAt) {
		node = node.next
	}

	return node
}

func (a *RGATreeList) release(node *RGATreeListNode) {
	if a.last == node {
		a.last = node.prev
	}

	node.prev.next = node.next
	if node.next != nil {
		node.next.prev = node.prev
	}
	node.prev, node.next = nil, nil

	a.nodeMapByIndex.Delete(node.indexNode)
	delete(a.nodeMapByCreatedAt, node.CreatedAt().Key())

	if !node.isRemoved() {
		a.size--
	}
}

func (a *RGATreeList) insertAfter(
	prevCreatedAt time.Ticket,
	value Element,
	executedAt time.Ticket,
) {
	prevNode := a.findNextBeforeExecutedAt(prevCreatedAt, executedAt)
	newNode := newRGATreeListNodeAfter(prevNode, value)
	if prevNode == a.last {
		a.last = newNode
	}

	a.nodeMapByIndex.InsertAfter(prevNode.indexNode, newNode.indexNode)
	a.nodeMapByCreatedAt[value.CreatedAt().Key()] = newNode

	a.size++
}
