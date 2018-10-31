package multichan

import (
	"reflect"
	"testing"
)

func TestSimple(t *testing.T) {
	w := New(0)
	r := w.Reader()
	var got []int
	ready := make(chan struct{})
	go func() {
		for {
			g, ok := r.Read(nil)
			if !ok {
				break
			}
			got = append(got, g.(int))
		}
		close(ready)
	}()

	w.Write(1)
	w.Write(2)
	w.Write(3)
	w.Close()

	<-ready

	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("got %v, want [1, 2, 3]", got)
	}
}

func TestTwo(t *testing.T) {
	var (
		w      = New(0)
		r1     = w.Reader()
		r2     = w.Reader()
		got1   []int
		got2   []int
		ready1 = make(chan struct{})
		ready2 = make(chan struct{})
	)

	go func() {
		for {
			g, ok := r1.Read(nil)
			if !ok {
				break
			}
			got1 = append(got1, g.(int))
		}
		close(ready1)
	}()

	go func() {
		for {
			g, ok := r2.Read(nil)
			if !ok {
				break
			}
			got2 = append(got2, g.(int))
		}
		close(ready2)
	}()

	w.Write(1)
	w.Write(2)
	w.Write(3)
	w.Close()

	<-ready1
	<-ready2

	if !reflect.DeepEqual(got1, []int{1, 2, 3}) {
		t.Errorf("reader 1: got %v, want [1, 2, 3]", got1)
	}
	if !reflect.DeepEqual(got2, []int{1, 2, 3}) {
		t.Errorf("reader 2: got %v, want [1, 2, 3]", got2)
	}
}

func TestNBRead(t *testing.T) {
	w := New(0)
	r := w.Reader()
	got, ok := r.NBRead()
	if ok {
		t.Errorf("unexpected success from NBRead")
	}
	w.Write(1)
	got, ok = r.NBRead()
	if !ok {
		t.Errorf("unexpected failure from NBRead")
	}
	if got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestTypecheck(t *testing.T) {
	var ok bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				ok = true
			}
		}()
		w := New(0)
		w.Write("foo")
	}()
	if !ok {
		t.Errorf("typechecking failed")
	}
}

func Test100(t *testing.T) {
	w := New(0)
	r := w.Reader()

	go func() {
		for i := 1; i <= 100; i++ {
			g, ok := r.Read(nil)
			got := g.(int)
			if !ok {
				t.Error("unexpected end of stream")
			} else if got != i {
				t.Errorf("got %d, want %d", got, i)
			}
		}
		_, ok := r.Read(nil)
		if ok {
			t.Errorf("unexpected non-end of stream")
		}
	}()

	for i := 1; i <= 100; i++ {
		w.Write(i)
	}
	w.Close()
}

func TestTrim(t *testing.T) {
	w := New(0)

	w.Write(1)
	r := w.Reader()
	w.Write(2)
	got, ok := r.Read(nil)
	if !ok {
		t.Fatal("unexpected end of stream")
	}
	gotInt, ok := got.(int)
	if !ok {
		t.Fatalf("unexpected %T on read, want int", got)
	}
	if gotInt != 2 {
		t.Errorf("got %d, want 2", gotInt)
	}
}
