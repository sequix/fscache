package fscache

import (
	"os"
	"syscall"
	"time"
)

type fileInfoHeap []os.FileInfo

func (f fileInfoHeap) Len() int {
	return len(f)
}

func (f fileInfoHeap) Less(i, j int) bool {
	fis := f[i].Sys().(*syscall.Stat_t)
	fjs := f[j].Sys().(*syscall.Stat_t)
	fia := time.Unix(fis.Atim.Sec, fis.Atim.Nsec)
	fja := time.Unix(fjs.Atim.Sec, fjs.Atim.Nsec)
	return fia.Before(fja)
}

func (f fileInfoHeap) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func (f *fileInfoHeap) Push(x interface{}) {
	*f = append(*f, x.(os.FileInfo))
}

func (f *fileInfoHeap) Pop() interface{} {
	old := *f
	n := len(old)
	x := old[n-1]
	*f = old[0 : n-1]
	return x
}
