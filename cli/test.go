package main

import "runtime"

func main() {
	println(runtime.GOMAXPROCS(runtime.NumCPU()))
}
