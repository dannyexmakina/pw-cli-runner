package main

import (
	"fmt"
	"io"
	"sync"
)

type progressPrinter struct {
	mu sync.Mutex
	w  io.Writer
}

func newProgress(w io.Writer) *progressPrinter {
	return &progressPrinter{w: w}
}

func (p *progressPrinter) Printf(label, format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.w, "[%s] "+format+"\n", append([]any{label}, args...)...)
}
