package main

import "testing"

// rfc1035Buf reproduces the exact message layout from RFC 1035 section 4.1.4.
//
//	offset 20: F.ISI.ARPA        written out in full
//	offset 40: FOO.F.ISI.ARPA    label FOO + pointer to 20
//	offset 64: ARPA              bare pointer to 26 (mid-name)
//	offset 92: root              a single zero octet
func rfc1035Buf() []byte {
	buf := make([]byte, 128)

	// offset 20: F.ISI.ARPA, no compression. Occupies bytes 20..31.
	copy(buf[20:], []byte{
		1, 'F',
		3, 'I', 'S', 'I',
		4, 'A', 'R', 'P', 'A',
		0,
	})

	// offset 40: label FOO followed by a pointer to 20. Bytes 40..45.
	copy(buf[40:], []byte{3, 'F', 'O', 'O', 0xC0, 20})

	// offset 64: bare pointer to 26, which is the ARPA label *inside*
	// the name at 20 -- not the start of a name. Bytes 64..65.
	copy(buf[64:], []byte{0xC0, 26})

	// offset 92: the root domain name, a single zero octet.
	buf[92] = 0

	return buf
}

func TestParseNameRFC1035(t *testing.T) {
	buf := rfc1035Buf()

	tests := []struct {
		desc     string
		offset   int
		wantName string
		wantNext int
	}{
		{"full name, no compression", 20, "F.ISI.ARPA", 32},
		{"labels then pointer", 40, "FOO.F.ISI.ARPA", 46},
		{"bare pointer into mid-name", 64, "ARPA", 66},
		{"root", 92, "", 93},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, next, err := parseName(buf, tt.offset)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantName {
				t.Errorf("name = %q, want %q", got, tt.wantName)
			}
			// The offset assertion is the one that matters: it must land
			// exactly on the byte after the name, where QTYPE begins.
			if next != tt.wantNext {
				t.Errorf("next offset = %d, want %d", next, tt.wantNext)
			}
		})
	}
}

func TestParseNameMalformed(t *testing.T) {
	tests := []struct {
		desc   string
		buf    []byte
		offset int
	}{
		{"self-referential pointer", []byte{0xC0, 0x00}, 0},
		{"forward pointer", []byte{0xC0, 0x04, 0, 0, 1, 'A', 0}, 0},
		// Starting at 2: the pointer to 0 is backward so it recurses; at 0
		// the pointer to 2 is now forward, so the cycle breaks on hop two.
		{"two-pointer loop", []byte{0xC0, 0x02, 0xC0, 0x00}, 2},
		{"pointer truncated at buffer end", []byte{0xC0}, 0},
		{"label length overruns buffer", []byte{5, 'F', 'O', 'O'}, 0},
		{"name never terminates", []byte{3, 'F', 'O', 'O'}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if _, _, err := parseName(tt.buf, tt.offset); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// Regression guard for the QTYPE+QCLASS skip in parseNames. Without the +4
// this returns garbage names rather than an error, so it fails silently.
func TestParseNamesMultipleQuestions(t *testing.T) {
	buf := make([]byte, 12)                          // header
	buf = append(buf, encodeName("kindle.local")...) // bytes 12..25
	buf = append(buf, 0, 1, 0, 1)                    // QTYPE=A, QCLASS=IN
	buf = append(buf, 0xC0, 0x0C)                    // second question: pointer to 12
	buf = append(buf, 0, 1, 0, 1)                    // QTYPE=A, QCLASS=IN

	names, next, err := parseNames(buf, 12, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("got %d names, want 2: %v", len(names), names)
	}
	if names[0] != "kindle.local" || names[1] != "kindle.local" {
		t.Errorf("names = %v, want both kindle.local", names)
	}
	if next != len(buf) {
		t.Errorf("next = %d, want %d", next, len(buf))
	}
}
