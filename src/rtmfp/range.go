package rtmfp

import (
	"container/list"
	"fmt"
)

func min(a, b uint) uint {
	if a > b {
		return b
	} else {
		return a
	}
}

func max(a, b uint) uint {
	if a > b {
		return a
	} else {
		return b
	}
}

func MakeRange(pos, end uint) Range {
	return Range{Pos: pos, Len: end - pos}
}

type Range struct {
	Pos uint
	Len uint
}

func (self *Range) End() uint {
	return self.Pos + self.Len
}

func (self *Range) Equals(r Range) bool {
	return self.Pos == r.Pos && self.Len == r.Len
}

func (self *Range) Contain(Pos uint) bool {

	return Pos >= self.Pos && Pos < self.End()
}

func IsRangeIntersect(a, b Range) bool {

	x := min(a.Pos, b.Pos)
	y := max(a.End(), b.End())

	return y-x < a.Len+b.Len
}

func RangeIntersect(a, b Range) Range {
	if a.Pos <= b.Pos {

		if a.End() > b.Pos {
			return Range{Pos: b.Pos, Len: min(a.End(), b.End()) - b.Pos}
		} else {
			return Range{}
		}

	} else {
		return RangeIntersect(b, a)
	}

	return Range{}
}

func CanRangeMerge(a, b Range) bool {

	x := min(a.Pos, b.Pos)
	y := max(a.End(), b.End())

	return y-x <= a.Len+b.Len
}

func RangeMerge(a, b Range) Range {

	if a.Pos <= b.Pos {

		if a.End() >= b.Pos {
			return Range{Pos: a.Pos, Len: max(a.End(), b.End()) - a.Pos}
		} else {
			panic("ranges not intersect.")
		}

	} else {
		return RangeMerge(b, a)
	}

	return Range{}
}

type RangeQueue struct {
	ranges list.List
}

func (self *RangeQueue) concat() {

	e := self.ranges.Front()
	for e != nil && e.Next() != nil {

		if CanRangeMerge(e.Value.(Range), e.Next().Value.(Range)) {
			e.Value = RangeMerge(e.Value.(Range), e.Next().Value.(Range))
			self.ranges.Remove(e.Next())
		} else {
			e = e.Next()
		}
	}
}

func (self *RangeQueue) AddRange(r Range) {

	if r.Len == 0 {
		return
	}

	for e := self.ranges.Front(); e != nil; e = e.Next() {
		v := e.Value.(Range)
		if CanRangeMerge(v, r) {
			self.ranges.InsertBefore(r, e)
			self.concat()
			return
		} else if r.Pos < v.Pos {
			self.ranges.InsertBefore(r, e)
			return
		}
	}

	self.ranges.PushBack(r)
}

func (self *RangeQueue) SubstractRange(r Range) {
	for e := self.ranges.Front(); e != nil; {
		v := e.Value.(Range)

		ir := RangeIntersect(v, r)
		if ir.Len > 0 {

			if ir.Equals(v) {
				n := e.Next()
				self.ranges.Remove(e)
				e = n
			} else if ir.Pos != v.Pos {

				e.Value = Range{Pos: v.Pos, Len: ir.Pos - v.Pos}

				n := e
				e = e.Next()

				if ir.End() != v.End() {
					self.ranges.InsertAfter(Range{
						Pos: ir.End(),
						Len: v.End() - ir.End(),
					}, n)
				}
			} else {
				e.Value = Range{Pos: ir.End(), Len: v.End() - ir.End()}
				e = e.Next()
			}
		} else {
			e = e.Next()
		}

		if r.End() < v.Pos {
			break
		}
	}
}

func (self *RangeQueue) AddRangeQueue(rq *RangeQueue) {

	for e := rq.ranges.Front(); e != nil; e = e.Next() {
		self.AddRange(e.Value.(Range))
	}
}

func (self *RangeQueue) SubstractRangeQueue(rq *RangeQueue) {
	for e := rq.ranges.Front(); e != nil; e = e.Next() {
		self.SubstractRange(e.Value.(Range))
	}
}

func (self *RangeQueue) Equals(rq *RangeQueue) bool {

	if self.ranges.Len() != rq.ranges.Len() {
		return false
	}

	e1 := self.ranges.Front()
	e2 := rq.ranges.Front()

	if e1 == nil || e2 == nil {
		return e1 == e2
	}

	for {
		v1 := e1.Value.(Range)
		v2 := e2.Value.(Range)
		if !v1.Equals(v2) {
			return false
		}

		e1 = e1.Next()
		e2 = e2.Next()

		if e1 == nil || e2 == nil {
			if e1 == e2 {
				return true
			} else {
				return false
			}
		}
	}

	return false
}

func (self *RangeQueue) Contain(Pos uint) bool {
	for e := self.ranges.Front(); e != nil; e = e.Next() {
		v := e.Value.(Range)
		if v.Contain(Pos) {
			return true
		}
	}

	return false
}

func (self *RangeQueue) String() (s string) {

	s = "["

	for e := self.ranges.Front(); e != nil; e = e.Next() {
		v := e.Value.(Range)
		s += fmt.Sprintf("[%d, %d)", v.Pos, v.End())
	}

	s += "]"

	return s
}

func (self *RangeQueue) ToArray() (ra []Range) {

	ra = make([]Range, self.ranges.Len())
	for i, e := 0, self.ranges.Front(); e != nil; e, i = e.Next(), i+1 {

		ra[i] = e.Value.(Range)
	}

	return ra
}

func RangeQueueIntersect(a, b *RangeQueue) *RangeQueue {

	rq := &RangeQueue{}

	for e1 := a.ranges.Front(); e1 != nil; e1 = e1.Next() {
		for e2 := b.ranges.Front(); e2 != nil; e2 = e2.Next() {
			rq.AddRange(RangeIntersect(e1.Value.(Range), e2.Value.(Range)))
		}
	}

	return rq
}

func RangeQueueFromArray(ra []Range) (rq *RangeQueue) {
	rq = &RangeQueue{}

	for _, r := range ra {
		rq.ranges.PushBack(r)
	}

	return rq
}
