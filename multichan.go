package multichan

import (
	"fmt"
	"log"
	"reflect"
	"sync"
)

// W is the writing end of a one-to-many data channel.
type W struct {
	mu sync.Mutex

	zero     interface{}  // the zero value of this channel
	zerotype reflect.Type // the type of the zero value

	chans []chan<- interface{}
}

// R is the reading end of a one-to-many data channel.
type R struct {
	C    <-chan interface{}
	w    *W
	id   int
	zero interface{}
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
	for i, ch := range w.chans {
		if ch != nil {
			go func() {
				log.Printf("sending to reader %d", i)
				ch <- item
			}()
		}
	}
	w.mu.Unlock()
}

// Close closes the writing end of a multichan,
// signaling to readers that the stream has ended.
// Reading past the end of the stream produces the zero value that was passed to New.
func (w *W) Close() {
	w.mu.Lock()
	for i, ch := range w.chans {
		if ch != nil {
			log.Printf("closing channel to reader %d", i)
			close(ch)
			w.chans[i] = nil
		}
	}
	w.mu.Unlock()
}

// Reader adds a new reader to the multichan and returns it.
// Readers consume resources in the multichan and should be disposed of (with Dispose) when no longer needed.
func (w *W) Reader() *R {
	w.mu.Lock()
	defer w.mu.Unlock()

	id := len(w.chans)
	ch := make(chan interface{})
	w.chans = append(w.chans, ch)
	return &R{
		C:    ch,
		w:    w,
		id:   id,
		zero: w.zero,
	}
}

// Read reads the next item in the multichan.
// It blocks until an item is ready to read.
// If the multichan is closed and the last item has already been consumed,
// this returns the multichan's zero value (see New) and false.
// Otherwise it returns the next value and true.
func (r *R) Read() (interface{}, bool) {
	log.Printf("reading with reader %d", r.id)
	item, ok := <-r.C
	if !ok {
		return r.zero, false
	}
	return item, true
}

// NBRead does a non-blocking read on the multichan.
// If the multichan is closed and the last item has already been consumed,
// or if no next item is ready to read,
// this returns the multichan's zero value (see New) and false.
// Otherwise it returns the next value and true.
func (r *R) NBRead() (interface{}, bool) {
	select {
	case item, ok := <-r.C:
		if !ok {
			return r.zero, false
		}
		return item, true
	default:
		return r.zero, false
	}
}

// Dispose removes r from its multichan, freeing up resources.
// It is an error to make further method calls on r after Dispose.
func (r *R) Dispose() {
	r.w.mu.Lock()
	r.w.chans[r.id] = nil
	r.w.mu.Unlock()
}
