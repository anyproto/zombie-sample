package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
	"syscall"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	filename := "example.txt"

	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Failed to open file: %v\n", err)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Printf("Failed to get file info: %v\n", err)
		return
	}
	fileSize := fileInfo.Size()

	fileMmap, err := syscall.Mmap(
		int(file.Fd()),
		0,
		int(fileSize),
		syscall.PROT_READ,
		syscall.MAP_PRIVATE,
	)
	accessFile(fileMmap, int(fileSize))
	if err != nil {
		fmt.Printf("Failed to mmap file: %v\n", err)
		return
	}
	nonFileMMap := fillMmap()

	//defer syscall.Munmap(fileMmap)

	var wg sync.WaitGroup
	wg.Add(1)
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		}
	}()
	wg.Wait()
	accessFile(fileMmap, int(fileSize))
	for i := 0; i < len(nonFileMMap); i++ {
		println(nonFileMMap)
	}
}

func accessFile(file []byte, fileSize int) {
	for i := 0; i < 100; i++ {
		fmt.Println(file[rand.IntN(fileSize)])
	}
}

func mmapSyscall(offset int64, length int, prot, flags, fd int) ([]byte, error) {
	data, err := syscall.Mmap(fd, offset, length, prot, flags)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func fillMmap() []byte {
	pageSize := syscall.Getpagesize()
	size := 1024 * 1024 * 100

	data, err := mmapSyscall(0, size+pageSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON, -1)
	if err != nil {
		fmt.Printf("mmap failed: %v\n", err)
		return nil
	}
	//defer syscall.Munmap(data)

	for i := 0; i < 1024*1024*100; i++ {
		data[i] = byte(i % 256)
	}

	return data
}
