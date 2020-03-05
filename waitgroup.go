// Copyright © 2020 Genome Research Limited
// Author: Sendu Bala <sb10@sanger.ac.uk>.
//
//  This file is part of waitgroup.
//
//  Permission is hereby granted, free of charge, to any person obtaining a copy
//  of this software and associated documentation files (the "Software"), to
//  deal in the Software without restriction, including without limitation the
//  rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
//  sell copies of the Software, and to permit persons to whom the Software is
//  furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

/*
Package waitgroup is like sync.WaitGroup, but with a timeout and debugging to
show what you're waiting for if a WaitGroup gets stuck waiting for too long or
forever.

	import "github.com/sb10/waitgroup"

	wg := waitgroup.New()
	loc := wg.Add(1)
	go func() {
		defer wg.Done(loc)
		// ...
	}()
	loc2 := wg.Add(1)
	go func() {
		// ... forgot to do wg.Done(loc2), or goroutine gets stuck before the Done()
		// call occurs
	}()
	wg.Wait(5 * time.Second) // tells you loc2 wasn't done
*/
package waitgroup

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// Options lets users specify options for WaitGroups. Set the Opts variable to
// one of these to choose your options.
type Options struct {
	Logger      io.Writer
	loggerMutex *sync.Mutex
}

func (o *Options) Log(pattern string, args ...interface{}) {
	o.loggerMutex.Lock()
	defer o.loggerMutex.Unlock()
	fmt.Fprintf(o.Logger, pattern, args...)
}

// Opts are the global options applied to all WaitGroups. Default is for logs
// to go to STDERR. To change, set Opts.Logger once at the start of your app.
var Opts = &Options{
	Logger:      os.Stderr,
	loggerMutex: &sync.Mutex{},
}

// WaitGroup is like sync.WaitGroup, but the Wait() has a timeout that tells you
// what you're still waiting on.
type WaitGroup struct {
	wg    *sync.WaitGroup
	calls map[string]int
	mu    sync.RWMutex
}

// New returns a new WaitGroup.
func New() *WaitGroup {
	return &WaitGroup{
		wg:    &sync.WaitGroup{},
		calls: make(map[string]int),
		mu:    &sync.RWMutex{},
	}
}

// Add is like sync.WaitGroup.Add(), but returns a key. The key must eventually
// be passed to a corresponding Done() call if i was positive.
func (w *WaitGroup) Add(i int) string {
	_, file, line, _ := runtime.Caller(1)
	key := fmt.Sprintf("%s:%d", file, line)

	w.mu.Lock()
	defer w.mu.Unlock()
	w.wg.Add(i)
	w.calls[key] += i
	return key
}

// Done is like sync.WaitGroup.Done(), but takes a key returned by Add().
func (w *WaitGroup) Done(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wg.Done()
	if _, exists := w.calls[key]; exists {
		w.calls[key]--
		if w.calls[key] <= 0 {
			delete(w.calls, key)
		}
	}
}

// Wait is like sync.WaitGroup.Wait(), but takes a duration to wait for, after
// which it logs which Add() calls have not yet had a matching Done() call
// executed.
func (w *WaitGroup) Wait(wait time.Duration) {
	done := make(chan struct{})
	go func() {
		limit := time.After(wait)
		for {
			select {
			case <-done:
				return
			case <-limit:
				w.LogNotDone()
			}
		}
	}()
	w.wg.Wait()
	close(done)
}

// LogNotDone logs all cases where Add(i) was called, but i corresponding Done()
// calls have not yet been done.
func (w *WaitGroup) LogNotDone() {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if len(w.calls) == 0 {
		return
	}
	Opts.Log("\nWaitGroup currently waiting on:\n")
	for key, n := range w.calls {
		Opts.Log(" %s (%d outstanding)\n", key, n)
	}
}
