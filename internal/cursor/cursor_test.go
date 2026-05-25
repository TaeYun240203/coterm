package cursor

import "testing"

func TestDeltaFromLineCount(t *testing.T) {
	old := Cursor{LineCount: 2}
	delta, next, external := Delta(old, "a\nb\nc\nd\n")
	if external {
		t.Fatal("external changes detected")
	}
	if delta != "c\nd\n" {
		t.Fatalf("delta = %q", delta)
	}
	if next.LineCount != 4 {
		t.Fatalf("next line count = %d", next.LineCount)
	}
}

func TestSnapshotTailTruncatesFromHead(t *testing.T) {
	out, truncated := Tail("1\n2\n3\n4\n5\n", 3, 1024)
	if !truncated {
		t.Fatal("expected truncation")
	}
	if out != "3\n4\n5\n" {
		t.Fatalf("tail = %q", out)
	}
}
