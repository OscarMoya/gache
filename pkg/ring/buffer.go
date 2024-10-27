// package ring defines the main api for how to manage the shards minimizing the gc pressure
package ring

import (
	"encoding/binary"
	"errors"
)

// Constants to define the size of the block header

// in the ring buffer the blocs will follow the following structure
// | size (8B) | timestamp (8B) | hashed key (8B) | raw key len (8B) | data len (8B) | raw key (?) | data (?) |
// The size of the block is stored as means of doing a minimal check. Probably not needed or is better to use a
// checksum or hash of the block to verify the integrity of the block
// All the headers are normalized to 8 bytes to make it easier to read and write the data to avoid any alignment issues
// or downsizing from other architectures.

const (
	BlockHeaderLenSize       = 8 // length of the block in the header to make it easier to read
	BlockHeaderTimestampSize = 8 // timestamp of the block in the header to make it easier to read
	BlockHeaderHashedKeySize = 8 // hashed key of the block in the header to make it easier to read
	BlockHeaderRawKeyLenSize = 8 // length of the raw key in the header to make it easier to read
	BlockHeaderDataLenSize   = 8 // length of the data in the header to make it easier to read

	SizeHeaderStart = 0
	SizeHeaderEnd   = BlockHeaderLenSize

	TimestampHeaderStart = SizeHeaderEnd
	TimestampHeaderEnd   = TimestampHeaderStart + BlockHeaderTimestampSize

	HashedKeyHeaderStart = TimestampHeaderEnd
	HashedKeyHeaderEnd   = HashedKeyHeaderStart + BlockHeaderHashedKeySize

	RawKeyLenHeaderStart = HashedKeyHeaderEnd
	RawKeyLenHeaderEnd   = RawKeyLenHeaderStart + BlockHeaderRawKeyLenSize

	DataLenHeaderStart = RawKeyLenHeaderEnd
	DataLenHeaderEnd   = DataLenHeaderStart + BlockHeaderDataLenSize

	BlockHeaderSize = BlockHeaderLenSize + BlockHeaderTimestampSize + BlockHeaderHashedKeySize + BlockHeaderRawKeyLenSize + BlockHeaderDataLenSize
)

var InsufficientBufferSpace = errors.New("block is too big to fit in the buffer")
var BufferOutOfRange = errors.New("index is out of range")
var InconsistentBlockData = errors.New("inconsistent block data")

// RingBuffer is a struct that represents a ring buffer
type RingBuffer struct {

	// Slice of bytes representing the buffer itself. When writing data into it,
	// the buffer will be filled from the beginning to the end, and then it will
	// start overwriting the oldest data
	buffer []byte
	size   int // size of the buffer

	currPosition int // current offset of the (FIFO)
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
func (r *RingBuffer) Add(block Block) error {
	blockLen := block.Len()
	if blockLen > r.size {
		return InsufficientBufferSpace
	}

	// to make things easier, the headers will be written first in blob and then the key and the data
	headers := make([]byte, BlockHeaderSize)
	// populate the headers
	binary.LittleEndian.PutUint64(headers[SizeHeaderStart:SizeHeaderEnd], uint64(blockLen))
	binary.LittleEndian.PutUint64(headers[TimestampHeaderStart:TimestampHeaderEnd], uint64(block.Timestamp))
	binary.LittleEndian.PutUint64(headers[HashedKeyHeaderStart:HashedKeyHeaderEnd], uint64(block.HashedKey))
	binary.LittleEndian.PutUint64(headers[RawKeyLenHeaderStart:RawKeyLenHeaderEnd], uint64(len(block.RawKey)))
	binary.LittleEndian.PutUint64(headers[DataLenHeaderStart:DataLenHeaderEnd], uint64(len(block.Data)))
	_, err := r.write(headers)
	if err != nil {
		return err
	}

	_, err = r.write(block.RawKey)
	if err != nil {
		return err
	}
	_, err = r.write(block.Data)
	if err != nil {
		return err
	}

	return nil

}

// Get returns the block of data at the given index
func (r *RingBuffer) Get(index int) (Block, error) {
	if index > r.size {
		return Block{}, BufferOutOfRange
	}

	// Get the block of data at the given index
	b := Block{}
	// Getting the total size or failing
	blockSize := binary.LittleEndian.Uint64(r.read(index+HashedKeyHeaderStart, index+HashedKeyHeaderEnd))
	if blockSize > uint64(r.size) {
		return Block{}, InsufficientBufferSpace
	}
	// parsing timestamp
	timestamp := binary.LittleEndian.Uint64(r.read(index+TimestampHeaderStart, index+TimestampHeaderEnd))
	// parsing hashed key
	hashedKey := binary.LittleEndian.Uint64(r.read(index+HashedKeyHeaderStart, index+HashedKeyHeaderEnd))
	// parsing raw key length and data length
	rawKeyLen := binary.LittleEndian.Uint64(r.read(index+RawKeyLenHeaderStart, index+RawKeyLenHeaderEnd))
	// parsing data length
	dataLen := binary.LittleEndian.Uint64(r.read(index+DataLenHeaderStart, index+DataLenHeaderEnd))

	if blockSize != uint64(BlockHeaderSize+rawKeyLen+dataLen) {
		return Block{}, InconsistentBlockData
	}

	// Fill block information
	b.Timestamp = int64(timestamp)
	b.HashedKey = int64(hashedKey)
	b.RawKey = r.read(index+DataLenHeaderEnd, index+DataLenHeaderEnd+int(rawKeyLen))
	b.Data = r.read(index+DataLenHeaderEnd+int(rawKeyLen), index+DataLenHeaderEnd+int(rawKeyLen)+int(dataLen))

	return b, nil
}

func (r *RingBuffer) read(index, size int) []byte {

	toReturn := make([]byte, size)
	index = index % r.size // normalize the index to make Get function easier to use to
	//	pretend this is an infinite buffer instead of a ring buffer
	target := (index + size) % r.size // normalize again in case the block wraps around the buffer
	if target < index {
		// If the target position is less than the current position, it means that the block
		// will wrap around the buffer. the block will need to be read in two parts
		// First part will be from the current position to the end of the buffer
		copy(toReturn, r.buffer[index:])
		// Second part will be from the beginning of the buffer to the remaining space
		copy(toReturn[r.size-index:], r.buffer[0:target])
	} else {
		// If the target position is greater than the current position, it means that the block
		// will not wrap around the buffer. We can read the block in a single part
		copy(toReturn, r.buffer[index:target])
	}

	return toReturn
}

// write writes the given data to the buffer. It will overwrite the oldest data if there is not enough space
func (r *RingBuffer) write(data []byte) (int, error) {

	// Calculate the length of the block
	dataLen := len(data)

	// check if the block fits in the remaining space of the buffer
	targetPos := (r.currPosition + dataLen) % r.size
	if targetPos < r.currPosition {
		// If the target position is less than the current position, it means that the block
		// will wrap around the buffer. the block will need to be written in two parts
		// First part will be from the current position to the end of the buffer
		copy(r.buffer[r.currPosition:], data[:r.size-r.currPosition])
		// Second part will be from the beginning of the buffer to the remaining space
		copy(r.buffer[0:], data[r.size-r.currPosition:])
	} else {
		// If the target position is greater than the current position, it means that the block
		// will not wrap around the buffer. We can write the block in a single part
		copy(r.buffer[r.currPosition:], data)
	}

	// Update the current position
	r.currPosition = targetPos
	return dataLen, nil

}

// Intermediate struct to represent the block of data to be stored in the in the ring buffer
// to make things easier to manage from the upper layers and to keep the buffer as a simple byte array
// In this initial version, the block is heavy coupled with the buffer. It is possible to decouple it
// if a proper writer/reader is implemented to handle the serialization/deserialization of the block
// and how to write it in the buffer. But this will be saved for later.
type Block struct {
	Timestamp int64
	HashedKey int64
	RawKey    []byte
	Data      []byte
}

func (b Block) Len() int {
	return len(b.Data) + len(b.RawKey) + BlockHeaderSize
}
