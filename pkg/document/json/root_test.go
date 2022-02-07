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

package json_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yorkie-team/yorkie/pkg/document/json"
	"github.com/yorkie-team/yorkie/pkg/document/time"
	"github.com/yorkie-team/yorkie/test/helper"
)

func registerTextElementWithGarbage(fromPos, toPos json.RGATreeSplitNodePos, root *json.Root, text json.TextElement) {
	if !fromPos.Equal(toPos) {
		root.RegisterTextElementWithGarbage(text)
	}
}

func TestRoot(t *testing.T) {
	t.Run("garbage collection for array test", func(t *testing.T) {
		root := helper.TestRoot()
		ctx := helper.TextChangeContext(root)
		array := json.NewArray(json.NewRGATreeList(), ctx.IssueTimeTicket())

		array.Add(json.NewPrimitive(0, ctx.IssueTimeTicket()))
		array.Add(json.NewPrimitive(1, ctx.IssueTimeTicket()))
		array.Add(json.NewPrimitive(2, ctx.IssueTimeTicket()))
		assert.Equal(t, "[0,1,2]", array.Marshal())

		targetElement := array.Get(1)
		ticket := ctx.IssueTimeTicket()
		array.DeleteByCreatedAt(targetElement.CreatedAt(), &ticket)
		root.RegisterRemovedElementPair(array, targetElement)
		assert.Equal(t, "[0,2]", array.Marshal())
		assert.Equal(t, 1, root.GarbageLen())

		assert.Equal(t, 1, root.GarbageCollect(time.MaxTicket))
		assert.Equal(t, 0, root.GarbageLen())
	})

	t.Run("garbage collection for text test", func(t *testing.T) {
		root := helper.TestRoot()
		ctx := helper.TextChangeContext(root)
		text := json.NewText(json.NewRGATreeSplit(json.InitialTextNode()), ctx.IssueTimeTicket())

		fromPos, toPos := text.CreateRange(0, 0)
		text.Edit(fromPos, toPos, nil, "Hello World", ctx.IssueTimeTicket())
		registerTextElementWithGarbage(fromPos, toPos, root, text)
		assert.Equal(t, `"Hello World"`, text.Marshal())
		assert.Equal(t, 0, root.GarbageLen())

		fromPos, toPos = text.CreateRange(5, 10)
		text.Edit(fromPos, toPos, nil, "Yorkie", ctx.IssueTimeTicket())
		registerTextElementWithGarbage(fromPos, toPos, root, text)
		assert.Equal(t, `"HelloYorkied"`, text.Marshal())
		assert.Equal(t, 1, root.GarbageLen())

		fromPos, toPos = text.CreateRange(0, 5)
		text.Edit(fromPos, toPos, nil, "", ctx.IssueTimeTicket())
		registerTextElementWithGarbage(fromPos, toPos, root, text)
		assert.Equal(t, `"Yorkied"`, text.Marshal())
		assert.Equal(t, 2, root.GarbageLen())

		fromPos, toPos = text.CreateRange(6, 7)
		text.Edit(fromPos, toPos, nil, "", ctx.IssueTimeTicket())
		registerTextElementWithGarbage(fromPos, toPos, root, text)
		assert.Equal(t, `"Yorkie"`, text.Marshal())
		assert.Equal(t, 3, root.GarbageLen())

		// It contains code marked tombstone.
		// After calling the garbage collector, the node will be removed.
		nodeLen := len(text.Nodes())
		assert.Equal(t, 4, nodeLen)

		assert.Equal(t, 3, root.GarbageCollect(time.MaxTicket))
		assert.Equal(t, 0, root.GarbageLen())
		nodeLen = len(text.Nodes())
		assert.Equal(t, 1, nodeLen)
	})

	t.Run("garbage collection for fragments of text", func(t *testing.T) {
		type test struct {
			from    int
			to      int
			content string
			want    string
			garbage int
		}

		root := helper.TestRoot()
		ctx := helper.TextChangeContext(root)
		text := json.NewText(json.NewRGATreeSplit(json.InitialTextNode()), ctx.IssueTimeTicket())

		tests := []test{
			{from: 0, to: 0, content: "Yorkie", want: `"Yorkie"`, garbage: 0},
			{from: 4, to: 5, content: "", want: `"Yorke"`, garbage: 1},
			{from: 2, to: 3, content: "", want: `"Yoke"`, garbage: 2},
			{from: 0, to: 1, content: "", want: `"oke"`, garbage: 3},
		}

		for _, tc := range tests {
			fromPos, toPos := text.CreateRange(tc.from, tc.to)
			text.Edit(fromPos, toPos, nil, tc.content, ctx.IssueTimeTicket())
			registerTextElementWithGarbage(fromPos, toPos, root, text)
			assert.Equal(t, tc.want, text.Marshal())
			assert.Equal(t, tc.garbage, root.GarbageLen())
		}

		assert.Equal(t, 3, root.GarbageCollect(time.MaxTicket))
		assert.Equal(t, 0, root.GarbageLen())
	})

	t.Run("garbage collection for rich text test", func(t *testing.T) {
		root := helper.TestRoot()
		ctx := helper.TextChangeContext(root)
		richText := json.NewRichText(json.NewRGATreeSplit(json.InitialTextNode()), ctx.IssueTimeTicket())

		fromPos, toPos := richText.CreateRange(0, 0)
		richText.Edit(fromPos, toPos, nil, "Hello World", nil, ctx.IssueTimeTicket())
		registerTextElementWithGarbage(fromPos, toPos, root, richText)
		assert.Equal(t, `[{"attrs":{},"val":"Hello World"}]`, richText.Marshal())
		assert.Equal(t, 0, root.GarbageLen())

		fromPos, toPos = richText.CreateRange(6, 11)
		richText.Edit(fromPos, toPos, nil, "Yorkie", nil, ctx.IssueTimeTicket())
		registerTextElementWithGarbage(fromPos, toPos, root, richText)
		assert.Equal(t, `[{"attrs":{},"val":"Hello "},{"attrs":{},"val":"Yorkie"}]`, richText.Marshal())
		assert.Equal(t, 1, root.GarbageLen())

		fromPos, toPos = richText.CreateRange(0, 6)
		richText.Edit(fromPos, toPos, nil, "", nil, ctx.IssueTimeTicket())
		registerTextElementWithGarbage(fromPos, toPos, root, richText)
		assert.Equal(t, `[{"attrs":{},"val":"Yorkie"}]`, richText.Marshal())
		assert.Equal(t, 2, root.GarbageLen())

		// It contains code marked tombstone.
		// After calling the garbage collector, the node will be removed.
		nodeLen := len(richText.Nodes())
		assert.Equal(t, 3, nodeLen)

		assert.Equal(t, 2, root.GarbageCollect(time.MaxTicket))
		assert.Equal(t, 0, root.GarbageLen())

		nodeLen = len(richText.Nodes())
		assert.Equal(t, 1, nodeLen)
	})

	t.Run("garbage collection for container test", func(t *testing.T) {
		root := helper.TestRoot()
		ctx := helper.TextChangeContext(root)

		obj := root.Object()
		obj.Set("1", json.NewPrimitive(1, ctx.IssueTimeTicket()))
		arr := json.NewArray(json.NewRGATreeList(), ctx.IssueTimeTicket()).
			Add(json.NewPrimitive(1, ctx.IssueTimeTicket())).
			Add(json.NewPrimitive(2, ctx.IssueTimeTicket())).
			Add(json.NewPrimitive(3, ctx.IssueTimeTicket()))
		obj.Set("2", arr)
		obj.Set("3", json.NewPrimitive(3, ctx.IssueTimeTicket()))
		assert.Equal(t, `{"1":1,"2":[1,2,3],"3":3}`, root.Object().Marshal())

		ticket := ctx.IssueTimeTicket()
		deleted := obj.Delete("2", &ticket)
		root.RegisterRemovedElementPair(obj, deleted)
		assert.Equal(t, `{"1":1,"3":3}`, obj.Marshal())
		assert.Equal(t, 4, root.GarbageLen())

		assert.Equal(t, 4, root.GarbageCollect(time.MaxTicket))
		assert.Equal(t, 0, root.GarbageLen())

		ticket = ctx.IssueTimeTicket()
		deleted = obj.Delete("3", &ticket)
		root.RegisterRemovedElementPair(obj, deleted)
		assert.Equal(t, `{"1":1}`, obj.Marshal())
		assert.Equal(t, 1, root.GarbageLen())

		assert.Equal(t, 1, root.GarbageCollect(time.MaxTicket))
		assert.Equal(t, 0, root.GarbageLen())
	})
}
