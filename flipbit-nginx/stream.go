package main

type Stream struct {
	LocalPort int32
	Upstream []string
	Type string
}

type Streams []Stream
