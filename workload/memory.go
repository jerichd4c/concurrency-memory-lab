// Package workload contains the test load that runs inside the child
// process: memory that is reserved and touched page by page, and CPU work
// split across workers.
package workload

import (
	"os"
	"sync/atomic"
	"syscall"
)

// Region is an anonymous memory mapping reserved with mmap. Reserving it only
// grows the virtual memory of the process; the kernel assigns a physical page
// the first time each page is written (a page fault).
type Region struct {
	data []byte
}

// Reserve maps size bytes of private anonymous memory. The memory lives
// outside the Go heap, so the garbage collector never touches it.
func Reserve(size int) (*Region, error) {
	data, err := syscall.Mmap(-1, 0, size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_PRIVATE|syscall.MAP_ANON)
	if err != nil {
		return nil, err
	}
	return &Region{data: data}, nil
}

// Size returns the size of the region in bytes.
func (r *Region) Size() int {
	return len(r.data)
}

// Touch writes one byte in every page of the region, so the kernel has to
// back each page with RAM (or bring it back from swap). It returns the number
// of pages written and stops early when stop becomes true.
func (r *Region) Touch(stop *atomic.Bool) int {
	pageSize := os.Getpagesize()
	pages := 0
	for off := 0; off < len(r.data); off += pageSize {
		if pages%256 == 0 && stop.Load() {
			break
		}
		// A plain store, not data[off]++: reading first would map the
		// shared zero page and the write would then cause a second fault.
		r.data[off] = 1
		pages++
	}
	return pages
}

// Release unmaps the region and gives its memory back to the system.
func (r *Region) Release() error {
	if r.data == nil {
		return nil
	}
	err := syscall.Munmap(r.data)
	r.data = nil
	return err
}
