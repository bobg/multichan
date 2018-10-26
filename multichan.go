package multichan

import (
	"fmt"
	"reflect"
	"sync"
)

// W is the writing end of a one-to-many data channel.
type W struct {
	mu   sync.Mutex
	cond sync.Cond

	zero     interface{}  // the zero value of this channel
	zerotype reflect.Type // the type of the zero value

	closed bool

	items  []interface{} // items written and waiting to be read
	offset int           // position in the stream of items[0]

	readerpos []int // each reader's position in the stream
}

// R is the reading end of a one-to-many data channel.
type R struct {
	id  int
	w   *W
	pos int
}

// New produces a new multichan writer.
// Its argument is the zero value that readers will see
// when reading from a closed multichan,
// (or when non-blockingly reading from an unready multichan).
func New(zero interface{}) *W {
	w := &W{
		zero:     zero,
		zerotype: reflect.TypeOf(zero),
	}
	w.cond.L = &w.mu
	return w
}

// Write adds an item to the multichan.
// Its type must match
// (i.e., must be assignable to <https://golang.org/ref/spec#Assignability>)
// that of the zero value passed to New.
//
// Each item written to w remains in an internal queue until the last reader has consumed it.
// Readers added later to a multichan may miss items added earlier.
func (w *W) Write(item interface{}) {
	t := reflect.TypeOf(item)
	if !t.AssignableTo(w.zerotype) {
		panic(fmt.Sprintf("cannot write %s to multichan of %s", t, w.zerotype))
	}
	w.mu.Lock()
	w.items = append(w.items, item)
	w.cond.Broadcast()
	w.mu.Unlock()
}

// Close closes the writing end of a multichan,
// signaling to readers that the stream has ended.
// Reading past the end of the stream produces the zero value that was passed to New.
func (w *W) Close() {
	w.mu.Lock()
	w.closed = true
	w.cond.Broadcast()
	w.mu.Unlock()
}

// Reader adds a new reader to the multichan and returns it.
func (w *W) Reader() *R {
	w.mu.Lock()
	defer w.mu.Unlock()
	id := len(w.readerpos)
	w.readerpos = append(w.readerpos, 0)
	return &R{
		id:  id,
		w:   w,
		pos: w.offset,
	}
}

// w.mu is held
func (w *W) streamlen() int {
	return w.offset + len(w.items)
}

// w.mu is held
func (w *W) item(pos int) interface{} {
	return w.items[pos-w.offset]
}

// Read reads the next item in the multichan.
// It blocks until an item is ready to read.
// If the multichan is closed and the last item has already been consumed,
// this returns the multichan's zero value (see New) and false.
// Otherwise it returns the next value and true.
func (r *R) Read() (interface{}, bool) {
	r.w.mu.Lock()
	defer r.w.mu.Unlock()
	for r.pos >= r.w.streamlen() && !r.w.closed {
		r.w.cond.Wait()
	}
	if r.pos >= r.w.streamlen() {
		return r.w.zero, false
	}
	return r.doRead(), true
}

// NBRead does a non-blocking read on the multichan.
// If the multichan is closed and the last item has already been consumed,
// or if no next item is ready to read,
// this returns the multichan's zero value (see New) and false.
// Otherwise it returns the next value and true.
func (r *R) NBRead() (interface{}, bool) {
	r.w.mu.Lock()
	defer r.w.mu.Unlock()
	if r.pos >= r.w.streamlen() {
		return r.w.zero, false
	}
	return r.doRead(), true
}

// r.w.mu is held, r.w.streamlen() > r.pos
func (r *R) doRead() interface{} {
	result := r.w.item(r.pos)
	r.pos++
	r.w.readerpos[r.id] = r.pos

	min := r.w.streamlen()
	for _, p := range r.w.readerpos {
		if p < min {
			min = p
		}
	}
	if delta := min - r.w.offset; delta > 0 {
		r.w.items = r.w.items[delta:]
		r.w.offset += delta
	}

	return result
}
