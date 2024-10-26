// package ring defines the main api for how to manage the shards minimizing the gc pressure
package ring

import (
	"errors"
)

var BlockHeaderLenSize = 8 // length of the block in the header to make it easier to read

var InsufficientBufferSpace = errors.New("block is too big to fit in the buffer")
var BufferrOutOfRange = errors.New("index is out of range")

// RingBuffer is a struct that represents a ring buffer
type RingBuffer struct {

	// Slice of bytes representing the buffer itself. When writing data into it,
	// the buffer will be filled from the beginning to the end, and then it will
	// start overwriting the oldest data
	buffer []byte
	size   int // size of the buffer

	currPosition int // current offset of the (FIFO)

	// To make things easier, the interaction with the buffer will be via blocks.
	// This will help identifying where all the blocks of data are stored for an easy retrieval
	// Currently it is a map, but it could be an array or slice or some sort of index.
	blockIdx map[int]int
	// index of the block in the buffer
	timeIdx map[int64]int
}

// NewRingBuffer creates a new ring buffer with the given size
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buffer:       make([]byte, size),
		size:         size,
		currPosition: 0,
	}
}

// Add adds a block of data to the ring buffer. If there is not enough space in the buffer,
// it will overwrite the oldest data by starting from the beginning of the buffer.
// This receiver returns error if the block is too big to fit in the buffer
func (r *RingBuffer) Add(block *Block) error {

	// Calculate the length of the block
	blockLen := block.Len()

	// Check if the block fits in the buffer
	if blockLen > r.size {
		return InsufficientBufferSpace
	}

	// check if the block fits in the remaining space of the buffer
	targetPos := r.currPosition + blockLen%r.size
	if targetPos < r.currPosition {
		// If the target position is less than the current position, it means that the block
		// will wrap around the buffer. the block will need to be written in two parts
		// First part will be from the current position to the end of the buffer
		copy(r.buffer[r.currPosition:], block.AllData[:r.size-r.currPosition])
		// Second part will be from the beginning of the buffer to the remaining space
		copy(r.buffer[0:], block.AllData[r.size-r.currPosition:])
	} else {
		// If the target position is greater than the current position, it means that the block
		// will not wrap around the buffer. We can write the block in a single part
		copy(r.buffer[r.currPosition:], block.AllData)
	}

	// Update the current position
	r.currPosition = targetPos
	return nil

}

// Get returns the block of data at the given index
func (r *RingBuffer) Get(index int, dataSize int, b *Block) error {
	if index > r.size {
		return BufferrOutOfRange
	}

	b.AllData = make([]byte, dataSize)
	if index+dataSize > r.size {
		// the block wraps around the buffer
		// First part will be from the current position to the end of the buffer
		copy(b.AllData, r.buffer[index:])
		// Second part will be from the beginning of the buffer to the remaining space
		copy(b.AllData[r.size-index:], r.buffer[0:dataSize-(r.size-index)])
	} else {
		// If the target position is greater than the current position, it means that the block
		// will not wrap around the buffer. We can write the block in a single part
		copy(b.AllData, r.buffer[index:index+dataSize])
	}

	return nil
}

// Intermediate struct to represent the block of data to be stored in the in the ring buffer
// to make things easier to manage from the upper layers and to keep the buffer as a simple byte array
type Block struct {
	hashedKeyLen int    // length of the hashed key
	rawKeyLen    int    // length of the raw key
	dataLen      int    // length of the data
	AllData      []byte // All data packed together in a single byte array. The lens are used to serialize and deserialize the data
	CreatedAt    int64  // timestamp of the data add request in UnixMilli

}

// Len returns the length of the block. The CrateAt field is not included in the length as there
// is no real need to include it other than add it in the index
func (b *Block) Len() int {
	return b.hashedKeyLen + b.rawKeyLen + b.dataLen

}
