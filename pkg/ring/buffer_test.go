package ring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRingBuffer_Add(t *testing.T) {

	type fields struct {
		buffer       []byte
		size         int
		currPosition int
	}

	type testCase struct {
		name           string
		fields         fields
		blocks         []Block
		want           error
		expectedBuffer []byte
	}

	tests := []testCase{
		{
			name: "Add a block that fits in the buffer",
			fields: fields{
				buffer:       make([]byte, 56),
				size:         56,
				currPosition: 0,
			},
			blocks: []Block{
				{
					Timestamp: 123456,
					HashedKey: 123456,
					RawKey:    []byte{5, 154, 201, 220, 118, 239, 171, 24},
					Data:      []byte{1, 2, 3, 4, 5, 6, 7, 8},
				},
			},
			want:           nil,
			expectedBuffer: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RingBuffer{
				buffer:       tt.fields.buffer,
				size:         tt.fields.size,
				currPosition: tt.fields.currPosition,
			}
			for _, block := range tt.blocks {
				err := r.Add(block)
				require.NoError(t, err)
			}
		})
	}

}
