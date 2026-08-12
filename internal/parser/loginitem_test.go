package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBTMDump(t *testing.T) {
	dump := []byte(`#1:
UUID: 1234
Name: Example Helper
Developer Name: Example Inc.
Type: background item
Identifier: com.example.helper
URL: file:///Applications/Example.app

#2:
UUID: 5678
Name: Login App
Type: login item
Identifier: com.example.login
URL: file:///Applications/Login%20App.app
`)
	items := ParseBTMDump(dump)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].BundleID != "com.example.helper" || !items[0].HasCategory("background") {
		t.Fatalf("unexpected background item: %#v", items[0])
	}
	if items[1].Program != "/Applications/Login App.app" {
		t.Fatalf("unexpected decoded path: %s", items[1].Program)
	}
}

func TestParseBTMMacOSFixtures(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{name: "macos-13.txt", count: 1},
		{name: "macos-14.txt", count: 2},
		{name: "macos-15.txt", count: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "btm", test.name))
			if err != nil {
				t.Fatal(err)
			}
			items := ParseBTMDump(data)
			if len(items) != test.count {
				t.Fatalf("expected %d items, got %d", test.count, len(items))
			}
			for _, item := range items {
				if item.ID == "" || item.Label == "" || item.Program == "" {
					t.Fatalf("fixture produced incomplete item: %#v", item)
				}
			}
		})
	}
}

func BenchmarkParseBTMDump(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "btm", "macos-14.txt"))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		ParseBTMDump(data)
	}
}
