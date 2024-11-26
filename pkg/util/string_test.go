package util

import (
	"regexp"
	"testing"
)

func TestGenerateHex(t *testing.T) {
	sr := NewSecureRandom()

	tests := []struct {
		name   string
		length int
	}{
		{"zero length", 0},
		{"odd length", 5},
		{"even length", 10},
		{"large length", 100},
	}

	hexPattern := regexp.MustCompile("^[0-9A-F]*$")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sr.GenerateHex(tt.length)
			if err != nil {
				t.Errorf("GenerateHex(%d) error = %v", tt.length, err)
				return
			}

			if len(result) != tt.length {
				t.Errorf("GenerateHex(%d) length = %d, want %d", tt.length, len(result), tt.length)
			}

			if !hexPattern.MatchString(result) {
				t.Errorf("GenerateHex(%d) = %s, not valid hex", tt.length, result)
			}
		})
	}
}

func TestGenerateHexUniqueness(t *testing.T) {
	sr := NewSecureRandom()
	seen := make(map[string]bool)
	iterations := 1000
	length := 10

	for i := 0; i < iterations; i++ {
		result, err := sr.GenerateHex(length)
		if err != nil {
			t.Errorf("GenerateHex error on iteration %d: %v", i, err)
			continue
		}

		if seen[result] {
			t.Errorf("Duplicate value generated: %s", result)
		}
		seen[result] = true
	}
}

func BenchmarkGenerateHex(b *testing.B) {
	sr := NewSecureRandom()
	lengths := []int{10, 20, 50, 100}

	for _, length := range lengths {
		b.Run(fmt.Sprintf("length_%d", length), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := sr.GenerateHex(length)
				if err != nil {
					b.Errorf("GenerateHex error: %v", err)
				}
			}
		})
	}
}