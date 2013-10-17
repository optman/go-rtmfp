package rtmfp

import (
	"testing"
)

func TestDataChunkHeap(t *testing.T) {

	heap := create_data_chunk_heap()

	heap.push(&data_chunk{seqNum: 2})
	heap.push(&data_chunk{seqNum: 3})
	heap.push(&data_chunk{seqNum: 1})

	if heap.pop().seqNum != 1 || heap.Len() != 2 {
		t.Fatal()
	}

	if heap.pop().seqNum != 2 || heap.Len() != 1 {
		t.Fatal()
	}

	if heap.pop().seqNum != 3 || heap.Len() != 0 {
		t.Fatal()
	}
}
