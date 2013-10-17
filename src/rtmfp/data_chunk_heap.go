package rtmfp

import (
	"container/heap"
)

type data_chunk_heap struct {
	chunks []*data_chunk
}

func (self *data_chunk_heap) Len() int {
	return len(self.chunks)
}

func (self *data_chunk_heap) Less(i, j int) bool {
	return self.chunks[i].seqNum < self.chunks[j].seqNum
}

func (self *data_chunk_heap) Swap(i, j int) {
	temp := self.chunks[i]
	self.chunks[i] = self.chunks[j]
	self.chunks[j] = temp
}

func (self *data_chunk_heap) Push(x interface{}) {
	self.chunks = append(self.chunks, x.(*data_chunk))
}

func (self *data_chunk_heap) Pop() interface{} {

	index := len(self.chunks) - 1
	last_e := self.chunks[index]

	//should i matter to do this?
	//would the go runtime smart enought to know this element should be GC?
	self.chunks[index] = nil

	//remove the last element.
	//is there a better way to shrink the array?
	self.chunks = self.chunks[:index]

	return last_e
}

func (self *data_chunk_heap) push(buf *data_chunk) {
	heap.Push(self, buf)
}

func (self *data_chunk_heap) pop() *data_chunk {
	return heap.Pop(self).(*data_chunk)
}

func create_data_chunk_heap() *data_chunk_heap {
	return &data_chunk_heap{chunks: make([]*data_chunk, 0)}
}
