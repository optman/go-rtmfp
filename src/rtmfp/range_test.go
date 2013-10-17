package rtmfp

import (
	"fmt"
	"testing"
)

func test_range(r1, r2 Range, t *testing.T) {
	if !r1.Equals(r2) {
		t.Fatal()
	}
}

func make_range(pos, end uint) Range {
	return Range{
		Pos: pos,
		Len: end - pos,
	}
}

func TestRange(t *testing.T) {
	r := make_range(0, 1)
	test_range(r, r, t)

	if IsRangeIntersect(make_range(0, 1), make_range(1, 2)) {
		t.Fatal()
	}

	if !IsRangeIntersect(make_range(0, 1), make_range(0, 2)) {
		t.Fatal()
	}

	ir := RangeIntersect(make_range(0, 1), make_range(1, 2))
	test_range(ir, make_range(0, 0), t)

	ir = RangeIntersect(make_range(0, 1), make_range(0, 2))
	test_range(ir, make_range(0, 1), t)

	ir = RangeIntersect(make_range(0, 3), make_range(1, 2))
	test_range(ir, make_range(1, 2), t)

	ir = RangeIntersect(make_range(0, 3), make_range(1, 4))
	test_range(ir, make_range(1, 3), t)

	ir = RangeIntersect(make_range(1, 4), make_range(0, 3))
	test_range(ir, make_range(1, 3), t)

	if !CanRangeMerge(make_range(0, 1), make_range(1, 3)) {
		t.Fatal()
	}

	if !CanRangeMerge(make_range(0, 1), make_range(0, 3)) {
		t.Fatal()
	}

	if CanRangeMerge(make_range(0, 1), make_range(3, 4)) {
		t.Fatal()
	}

	mr := RangeMerge(make_range(0, 1), make_range(1, 3))
	test_range(mr, make_range(0, 3), t)

	mr = RangeMerge(make_range(0, 1), make_range(0, 3))
	test_range(mr, make_range(0, 3), t)

	mr = RangeMerge(make_range(0, 3), make_range(2, 5))
	test_range(mr, make_range(0, 5), t)
}

func TestRangeQueue(t *testing.T) {

	rq := &RangeQueue{}

	rq.AddRange(make_range(0, 1))
	rq.AddRange(make_range(2, 3))
	rq.AddRange(make_range(1, 2))

	if rq.ranges.Len() != 1 {
		t.Fatal()
	}

	test_range(rq.ranges.Front().Value.(Range), make_range(0, 3), t)

	rq.AddRange(make_range(4, 10))
	if rq.ranges.Len() != 2 {
		t.Fatal()
	}

	rq.AddRange(make_range(2, 5))
	if rq.ranges.Len() != 1 {
		t.Fatal()
	}

	test_range(rq.ranges.Front().Value.(Range), make_range(0, 10), t)

	rq.SubstractRange(make_range(3, 8))
	rq.SubstractRange(make_range(3, 8))
	rq.SubstractRange(make_range(100, 108))
	if rq.ranges.Len() != 2 {
		t.Fatal()
	}

	test_range(rq.ranges.Front().Value.(Range), make_range(0, 3), t)
	test_range(rq.ranges.Front().Next().Value.(Range), make_range(8, 10), t)

	test_range(rq.ranges.Front().Next().Value.(Range), make_range(8, 10), t)

	rq2 := &RangeQueue{}
	rq2.AddRange(make_range(1, 5))
	rq2.AddRange(make_range(8, 11))

	rq3 := &RangeQueue{}
	rq3.AddRange(make_range(1, 3))
	rq3.AddRange(make_range(8, 10))

	rq4 := RangeQueueIntersect(rq, rq2)
	if !rq4.Equals(rq3) {
		fmt.Println(rq4.String())
		t.Fatal()
	}

	rq4.SubstractRangeQueue(rq3)
	if rq4.ranges.Len() != 0{
		fmt.Println(rq4.String())
		t.Fatal()
	}

	rq4.SubstractRangeQueue(rq3)
	if rq4.ranges.Len() != 0{
		fmt.Println(rq4.String())
		t.Fatal()
	}
}
